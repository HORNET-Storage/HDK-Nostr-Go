package connmgr

import (
	"context"
	"fmt"
	"log"

	"github.com/fxamacker/cbor/v2"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

func UploadDag(ctx context.Context, connectionManager ConnectionManager, dag *merkle_dag.Dag, publicKey *string, signature *string, progressChan chan<- types.UploadProgress) error {
	for connectionID := range connectionManager.ListConnections() {
		err := UploadDagSingle(ctx, connectionManager, connectionID, dag, publicKey, signature, progressChan)
		if err != nil {
			return fmt.Errorf("failed to upload DAG to node %s: %w", connectionID, err)
		}
	}

	return nil
}

func UploadDagSingle(ctx context.Context, connectionManager ConnectionManager, connectionID string, dag *merkle_dag.Dag, publicKey *string, signature *string, progressChan chan<- types.UploadProgress) error {
	stream, err := connectionManager.GetStream(ctx, connectionID, UploadID)
	if err != nil {
		return fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	encoder := cbor.NewEncoder(stream)
	totalLeafs := len(dag.Leafs)
	leafsSent := 0

	err = dag.IterateDag(func(leaf *merkle_dag.DagLeaf, parent *merkle_dag.DagLeaf) error {
		err := sendLeaf(ctx, stream, encoder, leaf, parent, dag, publicKey, signature)

		if err != nil {
			if progressChan != nil {
				progressChan <- types.UploadProgress{ConnectionID: connectionID, LeafsSent: leafsSent, TotalLeafs: totalLeafs, Error: err}
			}

			return err
		}

		leafsSent++

		if progressChan != nil {
			progressChan <- types.UploadProgress{ConnectionID: connectionID, LeafsSent: leafsSent, TotalLeafs: totalLeafs}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate and send DAG: %w", err)
	}

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

		log.Println("Sending leaf")
		if err := encoder.Encode(message); err != nil {
			return err
		}

		log.Println("Waiting for response")
		if result := WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("did not receive a valid response")
		}
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

		if result := WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("did not recieve a valid response")
		}
	}

	return nil
}
