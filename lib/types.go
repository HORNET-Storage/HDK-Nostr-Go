package lib

import (
	"context"

	merkle_dag "github.com/HORNET-Storage/Scionic-Merkle-Tree/dag"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

type MessageEnvelope struct {
	Type    string      `cbor:"type"`
	Payload interface{} `cbor:"payload"`
}

type UploadMessage struct {
	Root      string
	Packet    merkle_dag.SerializableTransmissionPacket
	PublicKey string
	Signature string
}

type DownloadMessage struct {
	Root      string
	PublicKey string
	Signature string
	Filter    *DownloadFilter
}

type LeafLabelRange struct {
	From int
	To   int
}

type DownloadFilter struct {
	LeafRanges     *LeafLabelRange
	IncludeContent bool // IncludeContent from LeafLabelRange always overrides this
}

type QueryFilter struct {
	Names   []string
	PubKeys []string
	Tags    map[string]string
}

type QueryMessage struct {
	QueryFilter map[string]string
}

type AdvancedQueryMessage struct {
	Filter QueryFilter
}

type QueryResponse struct {
	Hashes []string
}

type DagData struct {
	PublicKey secp256k1.PublicKey
	Signature schnorr.Signature
	Dag       merkle_dag.Dag
}

type BlockData struct {
	Leaf   merkle_dag.DagLeaf
	Branch merkle_dag.ClassicTreeBranch
}

type ResponseMessage struct {
	Ok bool
}

type ErrorMessage struct {
	Message string
}

type TagFilter struct {
	Tags    map[string]string
	OrderBy string
}

type Stream interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
	Context() context.Context
}

type Connector interface {
	Connect(ctx context.Context) error
	Disconnect() error
	OpenStream(ctx context.Context, protocolID string) (Stream, error)
}

type UploadProgress struct {
	ConnectionID string
	LeafsSent    int
	TotalLeafs   int
	Error        error
}

type DownloadProgress struct {
	ConnectionID   string
	LeafsRetreived int
	Error          error
}
