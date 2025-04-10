package connmgr

import (
	"context"
	"encoding/hex"
	"fmt"

	merkle_dag "github.com/HORNET-Storage/Scionic-Merkle-Tree/dag"
	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/signing"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func DownloadDag(ctx context.Context, connectionManager ConnectionManager, connectionID string, root string, privatekey *secp256k1.PrivateKey, filter *types.DownloadFilter, progressChan chan<- types.DownloadProgress) (context.Context, *types.DagData, error) {
	stream, err := connectionManager.GetStream(ctx, connectionID, DownloadID)
	if err != nil {
		return ctx, nil, fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	var publicKey *string = nil
	var signature *string = nil

	// Because this is a request and not an upload, we don't always care about the request being signed, signed requests are for locked resources etc
	if privatekey != nil {
		sig, err := signing.SignSerializedCid(root, privatekey)
		if err != nil {
			return ctx, nil, err
		}

		serializedSignature := sig.Serialize()

		serializedPubkey, err := signing.SerializePublicKey(privatekey.PubKey())
		if err != nil {
			return ctx, nil, err
		}

		err = signing.VerifySerializedCIDSignature(sig, root, privatekey.PubKey())
		if err != nil {
			return ctx, nil, err
		}

		encodedSignature := hex.EncodeToString(serializedSignature)

		publicKey = serializedPubkey
		signature = &encodedSignature
	}

	downloadMessage := types.DownloadMessage{
		Root: root,
	}

	if signature != nil {
		downloadMessage.PublicKey = *publicKey
		downloadMessage.Signature = *signature
	}

	if filter != nil {
		downloadMessage.Filter = filter
	}

	if err := WriteMessageToStream(stream, downloadMessage); err != nil {
		return ctx, nil, err
	}

	message, err := WaitForUploadMessage(stream)
	if err != nil {
		return ctx, nil, err
	}

	dagPublicKey, err := signing.DeserializePublicKey(message.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	signatureBytes, err := hex.DecodeString(message.Signature)
	if err != nil {
		return nil, nil, err
	}

	dagSignature, err := schnorr.ParseSignature(signatureBytes)
	if err != nil {
		return nil, nil, err
	}

	err = signing.VerifySerializedCIDSignature(dagSignature, message.Root, dagPublicKey)
	if err != nil {
		return nil, nil, err
	}

	dag := &merkle_dag.Dag{
		Root:  message.Root,
		Leafs: make(map[string]*merkle_dag.DagLeaf),
	}

	packet := merkle_dag.TransmissionPacketFromSerializable(&message.Packet)

	err = packet.Leaf.VerifyRootLeaf()
	if err != nil {
		return ctx, nil, err
	}

	dag.ApplyTransmissionPacket(packet)

	err = dag.Verify()
	if err != nil {
		return ctx, nil, err
	}

	err = WriteResponseToStream(stream, true)
	if err != nil {
		return ctx, nil, err
	}

	for {
		message, err := WaitForUploadMessage(stream)
		if err != nil {
			return ctx, nil, err
		}
		packet := merkle_dag.TransmissionPacketFromSerializable(&message.Packet)

		err = packet.Leaf.VerifyLeaf()
		if err != nil {
			return ctx, nil, err
		}

		dag.ApplyTransmissionPacket(packet)

		err = dag.Verify()
		if err != nil {
			return ctx, nil, err
		}

		err = WriteResponseToStream(stream, true)
		if err != nil {
			return ctx, nil, err
		}

		if progressChan != nil {
			progressChan <- types.DownloadProgress{ConnectionID: connectionID, LeafsRetreived: len(dag.Leafs)}
		}

		if len(dag.Leafs) >= (dag.Leafs[dag.Root].LeafCount + 1) {
			break
		}
	}

	err = dag.Verify()
	if err != nil {
		return ctx, nil, err
	}

	dagData := &types.DagData{
		PublicKey: *dagPublicKey,
		Signature: *dagSignature,
		Dag:       *dag,
	}

	return ctx, dagData, nil
}
