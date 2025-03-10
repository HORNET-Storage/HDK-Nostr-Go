package connmgr

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/fxamacker/cbor/v2"

	merkle_dag "github.com/HORNET-Storage/Scionic-Merkle-Tree/dag"
	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/signing"
)

func UploadDag(ctx context.Context, connectionManager ConnectionManager, dag *merkle_dag.Dag, privatekey *secp256k1.PrivateKey, progressChan chan<- types.UploadProgress) error {
	for connectionID := range connectionManager.ListConnections() {
		err := UploadDagSingle(ctx, connectionManager, connectionID, dag, privatekey, progressChan)
		if err != nil {
			return fmt.Errorf("failed to upload DAG to node %s: %w", connectionID, err)
		}
	}

	return nil
}

func UploadDagSingle(ctx context.Context, connectionManager ConnectionManager, connectionID string, dag *merkle_dag.Dag, privatekey *secp256k1.PrivateKey, progressChan chan<- types.UploadProgress) error {
	stream, err := connectionManager.GetStream(ctx, connectionID, UploadID)
	if err != nil {
		return fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	if privatekey == nil {
		return fmt.Errorf("unable to sign data due to missing private key")
	}

	signature, err := signing.SignData([]byte(dag.Root), privatekey)
	if err != nil {
		return fmt.Errorf("Failed to sign dag root")
	}

	serializedSignature := signature.Serialize()

	serializedPubkey, err := signing.SerializePublicKey(privatekey.PubKey())
	if err != nil {
		return fmt.Errorf("Failed to serialize pubkey")
	}

	err = signing.VerifySignature(signature, []byte(dag.Root), privatekey.PubKey())
	if err != nil {
		return fmt.Errorf("Failed to verify signature")
	}

	encoder := cbor.NewEncoder(stream)
	totalLeafs := len(dag.Leafs)
	leafsSent := 0
	sequence := dag.GetLeafSequence()

	for i, packet := range sequence {
		message := types.UploadMessage{
			Root:   dag.Root,
			Packet: *packet.ToSerializable(),
		}

		// Only add the pub key and signature to the first packet as that's what contains the root
		if i == 0 {
			message.PublicKey = *serializedPubkey
			message.Signature = hex.EncodeToString(serializedSignature)
		}

		if err := encoder.Encode(&message); err != nil {
			return err
		}

		if result := WaitForResponse(ctx, stream); !result {
			return fmt.Errorf("did not recieve a valid response")
		}

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
	}

	return nil
}
