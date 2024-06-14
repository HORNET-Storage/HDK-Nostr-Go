package original

import (
	"context"
	"log"

	"github.com/fxamacker/cbor/v2"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
)

// Download dag from single hornet node
func (client *Client) DownloadDag(ctx context.Context, root string, publicKey *string, signature *string, filter *types.DownloadFilter) (context.Context, *merkle_dag.Dag, error) {
	ctx, stream, err := client.openStream(ctx, DownloadV1)
	if err != nil {
		return ctx, nil, err
	}

	streamEncoder := cbor.NewEncoder(stream)

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
		log.Println("Failed to recieve upload message in time")

		return ctx, nil, err
	}

	log.Println("Recieved upload message")

	err = message.Leaf.VerifyRootLeaf()
	if err != nil {
		log.Println("Failed to verify root leaf")

		return ctx, nil, err
	}

	err = builder.AddLeaf(&message.Leaf, nil)
	if err != nil {
		return ctx, nil, err
	}

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

		log.Printf("Processed leaf: %s\n", message.Leaf.Hash)

		err = WriteResponseToStream(ctx, stream, true)
		if err != nil || !result {
			log.Println("Failed to write response to stream")

			break
		}
	}

	log.Println("Building and verifying dag")

	dag := builder.BuildDag(message.Root)

	err = dag.Verify()
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
