package connmgr

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"

	"github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

type Client struct {
	ip        *string
	port      *string
	publicKey *string

	Host          *host.Host
	ServerAddress *multiaddr.Multiaddr
	Peer          *peer.AddrInfo
}

const (
	UploadV1   protocol.ID = "/upload/1.0.0"
	DownloadV1 protocol.ID = "/download/1.0.0"
	BranchV1   protocol.ID = "/branch/1.0.0"
)

var Clients map[string]*Client

func init() {
	Clients = map[string]*Client{}
}

func Connect(ctx context.Context, ip string, port string, publicKey string) (context.Context, *Client, error) {
	host, err := libp2p.New()
	if err != nil {
		return ctx, nil, err
	}

	serverAddress := fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", ip, port, publicKey)
	maddr, err := multiaddr.NewMultiaddr(serverAddress)
	if err != nil {
		return ctx, nil, err
	}
	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return ctx, nil, err
	}
	if err := host.Connect(ctx, *serverInfo); err != nil {
		return ctx, nil, err
	}

	client := &Client{
		ip:        &ip,
		port:      &port,
		publicKey: &publicKey,

		Host:          &host,
		ServerAddress: &maddr,
		Peer:          serverInfo,
	}

	Clients[publicKey] = client

	log.Println("Connected to:", serverInfo)

	return ctx, client, nil
}

func Disconnect(publicKey string) error {
	client, err := GetClient(publicKey)
	if err != nil {
		return err
	}

	delete(Clients, publicKey)

	host := *client.Host

	host.Close()

	return nil
}

func GetClient(publicKey string) (*Client, error) {
	client, exists := Clients[publicKey]

	if !exists {
		return nil, fmt.Errorf("Host for this public key does not exist")
	}

	return client, nil
}

func (client *Client) Disconnect() error {
	delete(Clients, *client.publicKey)

	host := *client.Host

	host.Close()

	return nil
}

func (client *Client) openStream(ctx context.Context, protocol protocol.ID) (context.Context, network.Stream, error) {
	host := *client.Host

	stream, err := host.NewStream(ctx, client.Peer.ID, protocol)
	if err != nil {
		return ctx, nil, err
	}

	return ctx, stream, nil
}

// Upload dag to all connected hornet nodes
func UploadDag(ctx context.Context, dag *merkle_dag.Dag) (context.Context, error) {
	for _, client := range Clients {
		ctx, err := client.UploadDag(ctx, dag)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

// Upload dag to a single hornet node
func (client *Client) UploadDag(ctx context.Context, dag *merkle_dag.Dag) (context.Context, error) {
	ctx, stream, err := client.openStream(ctx, UploadV1)
	if err != nil {
		return nil, err
	}

	enc := cbor.NewEncoder(stream)
	count := len(dag.Leafs)

	rootLeaf := dag.Leafs[dag.Root]

	streamEncoder := cbor.NewEncoder(stream)

	encoding, _, err := multibase.Decode(dag.Root)
	if err != nil {
		log.Println("Failed to discover encoding")
		return nil, err
	}

	encoder := multibase.MustNewEncoder(encoding)

	err = dag.IterateDag(func(leaf *merkle_dag.DagLeaf, parent *merkle_dag.DagLeaf) {
		if leaf.Hash == dag.Root {
			message := types.UploadMessage{
				Root:  dag.Root,
				Count: count,
				Leaf:  *rootLeaf,
			}

			if err := enc.Encode(&message); err != nil {
				return //nil, err
			}

			log.Println("Uploaded root leaf")

			if result := WaitForResponse(ctx, stream); !result {
				stream.Close()

				return //ctx, fmt.Errorf("Did not recieve a valid response")
			}

			log.Println("Response received")
		} else {
			result, err := leaf.VerifyLeaf(encoder)
			if err != nil {
				log.Println("Failed to verify leaf")
				return //err
			}

			if !result {
				return //fmt.Errorf("Failed to verify leaf")
			}

			label := merkle_dag.GetLabel(leaf.Hash)

			var branch *merkle_dag.ClassicTreeBranch

			if len(leaf.Links) > 1 {
				branch, err = parent.GetBranch(label)
				if err != nil {
					log.Println("Failed to get branch")
					return //err
				}

				result, err = parent.VerifyBranch(branch)
				if err != nil {
					log.Println("Failed to verify branch")
					return //err
				}

				if !result {
					return //fmt.Errorf("Failed to verify branch for leaf")
				}
			}

			message := types.UploadMessage{
				Root:   dag.Root,
				Count:  count,
				Leaf:   *leaf,
				Parent: parent.Hash,
				Branch: branch,
			}

			if err := streamEncoder.Encode(&message); err != nil {
				log.Println("Failed to encode to stream")
				return //err
			}

			log.Println("Uploaded next leaf")

			if result = WaitForResponse(ctx, stream); !result {
				return //fmt.Errorf("Did not recieve a valid response")
			}

			log.Println("Response recieved")
		}
	})

	if err != nil {
		return nil, err
	}

	/*
			message := types.UploadMessage{
				Root:  dag.Root,
				Count: count,
				Leaf:  *rootLeaf,
			}

			if err := enc.Encode(&message); err != nil {
				return nil, err
			}

			log.Println("Uploaded root leaf")

			if result := WaitForResponse(ctx, stream); !result {
				stream.Close()

				return ctx, fmt.Errorf("Did not recieve a valid response")
			}

			log.Println("Response received")


		err = UploadLeafChildren(ctx, stream, rootLeaf, dag)
		if err != nil {
			log.Printf("Failed to upload leaf children: %e", err)

			stream.Close()

			return ctx, err
		}
	*/

	stream.Close()

	log.Println("Dag has been uploaded")

	return ctx, nil
}

func UploadLeafChildren(ctx context.Context, stream network.Stream, leaf *merkle_dag.DagLeaf, dag *merkle_dag.Dag) error {
	streamEncoder := cbor.NewEncoder(stream)

	encoding, _, err := multibase.Decode(dag.Root)
	if err != nil {
		log.Println("Failed to discover encoding")
		return err
	}

	encoder := multibase.MustNewEncoder(encoding)

	count := len(dag.Leafs)

	for label, hash := range leaf.Links {
		child, exists := dag.Leafs[hash]
		if !exists {
			return fmt.Errorf("Leaf with has does not exist in dag")
		}

		result, err := child.VerifyLeaf(encoder)
		if err != nil {
			log.Println("Failed to verify leaf")
			return err
		}

		if !result {
			return fmt.Errorf("Failed to verify leaf")
		}

		var branch *merkle_dag.ClassicTreeBranch

		if len(leaf.Links) > 1 {
			branch, err = leaf.GetBranch(label)
			if err != nil {
				log.Println("Failed to get branch")
				return err
			}

			result, err = leaf.VerifyBranch(branch)
			if err != nil {
				log.Println("Failed to verify branch")
				return err
			}

			if !result {
				return fmt.Errorf("Failed to verify branch for leaf")
			}
		}

		message := types.UploadMessage{
			Root:   dag.Root,
			Count:  count,
			Leaf:   *child,
			Parent: leaf.Hash,
			Branch: branch,
		}

		if err := streamEncoder.Encode(&message); err != nil {
			log.Println("Failed to encode to stream")
			return err
		}

		log.Println("Uploaded next leaf")

		if result = WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("Did not recieve a valid response")
		}

		log.Println("Response recieved")
	}

	for _, hash := range leaf.Links {
		child, exists := dag.Leafs[hash]
		if !exists {
			return fmt.Errorf("Leaf with hash does not exist in dag")
		}

		if len(child.Links) > 0 {
			err = UploadLeafChildren(ctx, stream, child, dag)
			if err != nil {
				log.Println("Failed to Upload Leaf Children")
				return err
			}
		}
	}

	return nil
}

// Download dag from single hornet node
func (client *Client) DownloadDag(ctx context.Context, root string) (context.Context, *merkle_dag.Dag, error) {
	ctx, stream, err := client.openStream(ctx, DownloadV1)
	if err != nil {
		return ctx, nil, err
	}

	streamEncoder := cbor.NewEncoder(stream)

	downloadMessage := types.DownloadMessage{
		Root: root,
	}

	if err := streamEncoder.Encode(&downloadMessage); err != nil {
		return ctx, nil, err
	}

	builder := merkle_dag.CreateDagBuilder()

	result, message := WaitForUploadMessage(ctx, stream)
	if !result {
		log.Println("Failed to recieve upload message in time")

		return ctx, nil, err
	}

	log.Println("Recieved upload message")

	encoding, _, err := multibase.Decode(message.Root)
	if err != nil {
		log.Println("Failed to discover encoding from root hash")

		return ctx, nil, err
	}

	encoder := multibase.MustNewEncoder(encoding)

	result, err = message.Leaf.VerifyRootLeaf(encoder)
	if err != nil || !result {
		log.Println("Failed to verify root leaf")

		return ctx, nil, err
	}

	builder.AddLeaf(&message.Leaf, encoder, nil)

	log.Println("Processed root leaf")

	err = WriteResponseToStream(ctx, stream, true)
	if err != nil || !result {
		log.Println("Failed to write response to stream")

		return ctx, nil, err
	}

	for {
		log.Println("Waiting for upload message")

		result, message := WaitForUploadMessage(ctx, stream)
		if !result {
			log.Println("Failed to recieve upload message in time")

			break
		}

		log.Println("Recieved upload message")

		encoding, _, err := multibase.Decode(message.Root)
		if err != nil {
			log.Println("Failed to discover encoding from root hash")

			break
		}

		encoder := multibase.MustNewEncoder(encoding)

		result, err = message.Leaf.VerifyLeaf(encoder)
		if err != nil || !result {
			log.Println("Failed to verify leaf")

			break
		}

		parent, exists := builder.Leafs[message.Parent]
		if err != nil || !exists {
			log.Println("Failed to find parent leaf")

			break
		}

		if message.Branch != nil {
			result, err = parent.VerifyBranch(message.Branch)
			if err != nil || !result {
				log.Println("Failed to verify leaf branch")

				break
			}
		}

		builder.AddLeaf(&message.Leaf, encoder, parent)

		log.Printf("Processed leaf: %s\n", message.Leaf.Hash)

		err = WriteResponseToStream(ctx, stream, true)
		if err != nil || !result {
			log.Println("Failed to write response to stream")

			break
		}
	}

	log.Println("Building and verifying dag")

	dag := builder.BuildDag(message.Root)

	result, err = dag.Verify(encoder)
	if err != nil {
		log.Println("Failed to verify dag")

		return ctx, nil, err
	}

	if !result {
		log.Printf("Failed to verify dag: %s\n", message.Root)
	}

	log.Println("Download finished")

	return ctx, dag, nil
}

func WaitForResponse(ctx context.Context, stream network.Stream) bool {
	streamDecoder := cbor.NewDecoder(stream)

	var response types.ResponseMessage

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return false
		default:
			if err := streamDecoder.Decode(&response); err == nil {
				break wait
			}
		}
	}

	if !response.Ok {
		return false
	}

	return true
}

func WaitForUploadMessage(ctx context.Context, stream network.Stream) (bool, *lib.UploadMessage) {
	streamDecoder := cbor.NewDecoder(stream)

	var message types.UploadMessage

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return false, nil
		default:
			err := streamDecoder.Decode(&message)

			if err != nil {
				log.Printf("Error reading from stream: %e", err)
			}

			if err == io.EOF {
				return false, nil
			}

			if err == nil {
				break wait
			}
		}
	}

	return true, &message
}

func WriteResponseToStream(ctx context.Context, stream network.Stream, response bool) error {
	streamEncoder := cbor.NewEncoder(stream)

	message := types.ResponseMessage{
		Ok: response,
	}

	if err := streamEncoder.Encode(&message); err != nil {
		return err
	}

	return nil
}
