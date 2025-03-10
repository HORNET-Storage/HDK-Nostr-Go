package lib

import (
	"context"

	merkle_dag "github.com/HORNET-Storage/Scionic-Merkle-Tree/dag"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

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
	From           string
	To             string
	IncludeContent bool
}

type DownloadFilter struct {
	Leaves         []string
	LeafRanges     []LeafLabelRange
	IncludeContent bool // IncludeContent from LeafLabelRange always overrides this
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

type QueryFilter struct {
	Names   []string
	PubKeys []string
	Tags    map[string]string
}

type TagFilter struct {
	Tags    map[string]string
	OrderBy string
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
