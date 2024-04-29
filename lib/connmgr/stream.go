package connmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/fxamacker/cbor/v2"
	"github.com/libp2p/go-libp2p/core/network"
)

func WaitForResponse(ctx context.Context, stream network.Stream) bool {
	streamDecoder := cbor.NewDecoder(stream)

	var response types.ResponseMessage

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return false
		default:
			if err := streamDecoder.Decode(&response); err == nil {
				break wait
			}
		}
	}

	if !response.Ok {
		return false
	}

	return true
}

func WaitForUploadMessage(ctx context.Context, stream network.Stream) (bool, *lib.UploadMessage) {
	streamDecoder := cbor.NewDecoder(stream)

	var message types.UploadMessage

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return false, nil
		default:
			err := streamDecoder.Decode(&message)

			if err != nil {
				log.Printf("Error reading from stream: %e", err)
			}

			if err == io.EOF {
				return false, nil
			}

			if err == nil {
				break wait
			}
		}
	}

	return true, &message
}

func WriteResponseToStream(ctx context.Context, stream network.Stream, response bool) error {
	streamEncoder := cbor.NewEncoder(stream)

	message := types.ResponseMessage{
		Ok: response,
	}

	if err := streamEncoder.Encode(&message); err != nil {
		return err
	}

	return nil
}

func ReadMessageFromStream[T any](stream network.Stream) (*T, error) {
	streamDecoder := cbor.NewDecoder(stream)

	var message T

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("WaitForMessage timed out")
		default:
			err := streamDecoder.Decode(&message)

			if err != nil {
				return nil, err
			}

			if err == io.EOF {
				return nil, err
			}

			break wait
		}
	}

	return &message, nil
}

func WriteMessageToStream[T any](stream network.Stream, message T) error {
	enc := cbor.NewEncoder(stream)

	if err := enc.Encode(&message); err != nil {
		return err
	}

	return nil
}

func ReadJsonMessageFromStream[T any](stream network.Stream) (*T, error) {
	streamDecoder := json.NewDecoder(stream)

	var message T

	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("WaitForMessage timed out")
		default:
			err := streamDecoder.Decode(&message)

			if err != nil {
				return nil, err
			}

			if err == io.EOF {
				return nil, err
			}

			break wait
		}
	}

	return &message, nil
}

func WriteJsonMessageToStream[T any](stream network.Stream, message T) error {
	enc := json.NewEncoder(stream)

	if err := enc.Encode(&message); err != nil {
		return err
	}

	return nil
}
