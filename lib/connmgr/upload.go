package connmgr

import (
	"context"
	"fmt"
	"log"

	"github.com/fxamacker/cbor/v2"
	"github.com/libp2p/go-libp2p/core/network"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

// Upload dag to all connected hornet nodes
func UploadDag(ctx context.Context, dag *merkle_dag.Dag, publicKey *string, signature *string) (context.Context, error) {
	for _, client := range Clients {
		ctx, err := client.UploadDag(ctx, dag, publicKey, signature)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil
}

// Upload dag to a single hornet node
func (client *Client) UploadDag(ctx context.Context, dag *merkle_dag.Dag, publicKey *string, signature *string) (context.Context, error) {
	ctx, stream, err := client.openStream(ctx, UploadV1)
	if err != nil {
		return nil, err
	}

	enc := cbor.NewEncoder(stream)
	count := len(dag.Leafs)

	rootLeaf := dag.Leafs[dag.Root]

	streamEncoder := cbor.NewEncoder(stream)

	err = dag.IterateDag(func(leaf *merkle_dag.DagLeaf, parent *merkle_dag.DagLeaf) error {
		if leaf.Hash == dag.Root {
			message := types.UploadMessage{
				Root:  dag.Root,
				Count: count,
				Leaf:  *rootLeaf,
			}

			if publicKey != nil {
				message.PublicKey = *publicKey
			}

			if signature != nil {
				message.Signature = *signature
			}

			if err := enc.Encode(&message); err != nil {
				return err
			}

			log.Println("Uploaded root leaf")

			if result := WaitForResponse(ctx, stream); !result {
				stream.Close()

				return fmt.Errorf("Did not recieve a valid response")
			}

			log.Println("Response received")
		} else {
			err := leaf.VerifyLeaf()
			if err != nil {
				log.Println("Failed to verify leaf")
				return err
			}

			label := merkle_dag.GetLabel(leaf.Hash)

			var branch *merkle_dag.ClassicTreeBranch

			if len(leaf.Links) > 1 {
				branch, err = parent.GetBranch(label)
				if err != nil {
					log.Println("Failed to get branch")
					return err
				}

				err = parent.VerifyBranch(branch)
				if err != nil {
					log.Println("Failed to verify branch")
					return err
				}
			}

			message := types.UploadMessage{
				Root:   dag.Root,
				Count:  count,
				Leaf:   *leaf,
				Parent: parent.Hash,
				Branch: branch,
			}

			if publicKey != nil {
				message.PublicKey = *publicKey
			}

			if signature != nil {
				message.Signature = *signature
			}

			if err := streamEncoder.Encode(&message); err != nil {
				return err
			}

			log.Println("Uploaded next leaf")

			if result := WaitForResponse(ctx, stream); !result {
				return fmt.Errorf("Did not recieve a valid response")
			}

			log.Println("Response recieved")
		}

		return nil
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

	count := len(dag.Leafs)

	for label, hash := range leaf.Links {
		child, exists := dag.Leafs[hash]
		if !exists {
			return fmt.Errorf("Leaf with has does not exist in dag")
		}

		err := child.VerifyLeaf()
		if err != nil {
			log.Println("Failed to verify leaf")
			return err
		}

		var branch *merkle_dag.ClassicTreeBranch

		if len(leaf.Links) > 1 {
			branch, err = leaf.GetBranch(label)
			if err != nil {
				log.Println("Failed to get branch")
				return err
			}

			err = leaf.VerifyBranch(branch)
			if err != nil {
				log.Println("Failed to verify branch")
				return err
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

		if result := WaitForResponse(ctx, stream); !result {
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
			err := UploadLeafChildren(ctx, stream, child, dag)
			if err != nil {
				log.Println("Failed to Upload Leaf Children")
				return err
			}
		}
	}

	return nil
}
