package connmgr

import (
	"context"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
)

func QueryDag(ctx context.Context, connectionManager ConnectionManager, connectionID string, query types.QueryFilter) ([]string, error) {
	stream, err := connectionManager.GetStream(ctx, connectionID, QueryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	streamEncoder := cbor.NewEncoder(stream)

	queryMessage := types.AdvancedQueryMessage{
		Filter: query,
	}

	if err := streamEncoder.Encode(&queryMessage); err != nil {
		return nil, err
	}

	message, err := ReadMessageFromStream[types.QueryResponse](stream)
	if err != nil {
		return nil, err
	}

	return message.Hashes, nil
}
