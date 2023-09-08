package connmgr

import (
	"context"
	"fmt"
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

	for _, leaf := range dag.Leafs {
		message := types.UploadMessage{
			Root:  dag.Root,
			Count: count,
			Leaf:  *leaf,
		}

		if err := enc.Encode(&message); err != nil {
			return nil, err
		}

		// Temporary solution to ensure all data gets written to the stream
		time.Sleep(10 * time.Millisecond)
	}

	stream.Close()

	return ctx, nil
}

func (client *Client) DownloadDag(ctx context.Context, root string) (context.Context, *merkle_dag.Dag, error) {
	ctx, stream, err := client.openStream(ctx, DownloadV1)
	if err != nil {
		return ctx, nil, err
	}

	enc := cbor.NewEncoder(stream)
	dec := cbor.NewDecoder(stream)

	message := types.DownloadMessage{
		Root: root,
	}

	if err := enc.Encode(&message); err != nil {
		return ctx, nil, err
	}

	var rootLeafMessage types.UploadMessage

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			stream.Close()
			return ctx, nil, fmt.Errorf("Failed to receieve root leaf message")
		default:
			if err := dec.Decode(&rootLeafMessage); err == nil {
				break wait
			}
		}
	}

	encoding, _, err := multibase.Decode(rootLeafMessage.Root)
	if err != nil {
		return ctx, nil, err
	}

	encoder := multibase.MustNewEncoder(encoding)

	result, err := rootLeafMessage.Leaf.VerifyLeaf(encoder)
	if err != nil {
		log.Fatal(err)
	}

	if !result {
		err := fmt.Errorf("Failed to verify root leaf: %s\n", rootLeafMessage.Leaf.Hash)

		stream.Close()
		return ctx, nil, err
	}

	builder := merkle_dag.CreateDagBuilder()

	builder.AddLeaf(&rootLeafMessage.Leaf, encoder, nil)

	return ctx, nil, nil
}
