package connmgr

import (
	"context"
	"fmt"
	"log"

	"github.com/fxamacker/cbor/v2"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

func DownloadDag(ctx context.Context, connectionManager ConnectionManager, connectionID string, root string, publicKey *string, signature *string, filter *types.DownloadFilter, progressChan chan<- types.DownloadProgress) (context.Context, *merkle_dag.Dag, error) {
	stream, err := connectionManager.GetStream(ctx, connectionID, DownloadV1)
	if err != nil {
		return ctx, nil, fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	streamEncoder := cbor.NewEncoder(stream)

	leafCount := 0

	downloadMessage := types.DownloadMessage{
		Root: root,
	}

	if publicKey != nil {
		downloadMessage.PublicKey = *publicKey
	}

	if signature != nil {
		downloadMessage.Signature = *signature
	}

	if filter != nil {
		downloadMessage.Filter = filter
	}

	if err := streamEncoder.Encode(&downloadMessage); err != nil {
		return ctx, nil, err
	}

	builder := merkle_dag.CreateDagBuilder()

	result, message := WaitForUploadMessage(ctx, stream)
	if !result {
		return ctx, nil, err
	}

	err = message.Leaf.VerifyRootLeaf()
	if err != nil {
		log.Println("Failed to verify root leaf")

		return ctx, nil, err
	}

	err = builder.AddLeaf(&message.Leaf, nil)
	if err != nil {
		return ctx, nil, err
	}

	err = WriteResponseToStream(ctx, stream, true)
	if err != nil || !result {
		log.Println("Failed to write response to stream")

		return ctx, nil, err
	}

	leafCount++

	if progressChan != nil {
		progressChan <- types.DownloadProgress{ConnectionID: connectionID, LeafsRetreived: leafCount}
	}

	for {
		result, message := WaitForUploadMessage(ctx, stream)
		if !result {
			break
		}

		err = message.Leaf.VerifyLeaf()
		if err != nil {
			log.Println("Failed to verify leaf")

			break
		}

		parent, exists := builder.Leafs[message.Parent]
		if !exists {
			log.Println("Failed to find parent leaf")

			break
		}

		if message.Branch != nil {
			err = parent.VerifyBranch(message.Branch)
			if err != nil {
				log.Println("Failed to verify leaf branch")

				break
			}
		}

		err = builder.AddLeaf(&message.Leaf, parent)
		if err != nil {
			log.Println("Failed to add leaf to builder")

			break
		}

		err = WriteResponseToStream(ctx, stream, true)
		if err != nil || !result {
			log.Println("Failed to write response to stream")

			break
		}

		leafCount++

		if progressChan != nil {
			progressChan <- types.DownloadProgress{ConnectionID: connectionID, LeafsRetreived: leafCount}
		}
	}

	dag := builder.BuildDag(message.Root)

	err = dag.Verify()
	if err != nil {
		log.Println("Failed to verify dag")

		return ctx, nil, err
	}

	if !result {
		log.Printf("Failed to verify dag: %s\n", message.Root)
	}

	return ctx, dag, nil
}
