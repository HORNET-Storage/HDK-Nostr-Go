package connmgr

import (
	"context"
	"fmt"
	"log"

	"github.com/fxamacker/cbor/v2"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

func UploadDag(ctx context.Context, connectionManager ConnectionManager, dag *merkle_dag.Dag, publicKey *string, signature *string) error {
	for connectionID := range connectionManager.ListConnections() { // Assuming a method to list all connections
		err := UploadDagSingle(ctx, connectionManager, connectionID, dag, publicKey, signature)
		if err != nil {
			return fmt.Errorf("failed to upload DAG to node %s: %w", connectionID, err)
		}
	}
	return nil
}

func UploadDagSingle(ctx context.Context, connectionManager ConnectionManager, connectionID string, dag *merkle_dag.Dag, publicKey *string, signature *string) error {
	stream, err := connectionManager.GetStream(ctx, connectionID, UploadV1)
	if err != nil {
		return fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	encoder := cbor.NewEncoder(stream)

	err = dag.IterateDag(func(leaf *merkle_dag.DagLeaf, parent *merkle_dag.DagLeaf) error {
		return sendLeaf(ctx, stream, encoder, leaf, parent, dag, publicKey, signature)
	})
	if err != nil {
		return fmt.Errorf("failed to iterate and send DAG: %w", err)
	}

	log.Println("DAG has been uploaded successfully")
	return nil
}

func sendLeaf(ctx context.Context, stream types.Stream, encoder *cbor.Encoder, leaf *merkle_dag.DagLeaf, parent *merkle_dag.DagLeaf, dag *merkle_dag.Dag, publicKey *string, signature *string) error {
	count := len(dag.Leafs)

	if leaf.Hash == dag.Root {
		message := types.UploadMessage{
			Root:  dag.Root,
			Count: count,
			Leaf:  *leaf,
		}

		if publicKey != nil {
			message.PublicKey = *publicKey
		}
		if signature != nil {
			message.Signature = *signature
		}

		if err := encoder.Encode(message); err != nil {
			return err
		}

		if result := WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("did not receive a valid response")
		}

		log.Printf("Uploaded leaf %s\n", leaf.Hash)
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

		if err := encoder.Encode(&message); err != nil {
			return err
		}

		log.Println("Uploaded next leaf")

		if result := WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("did not recieve a valid response")
		}

		log.Println("Response recieved")
	}

	return nil
}
