package connmgr

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"reflect"
	"time"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/fxamacker/cbor/v2"
)

func BuildErrorMessage(message string, err error) types.ErrorMessage {
	log.Println("\n====[STREAM ERROR MESSAGE]====")
	if len(message) > 0 {
		log.Println(message)
	}
	if err != nil {
		log.Println(err)
	}
	log.Println("")

	return types.ErrorMessage{
		Message: fmt.Sprintf(message, err),
	}
}

func BuildResponseMessage(response bool) types.ResponseMessage {
	return types.ResponseMessage{
		Ok: response,
	}
}

func WriteErrorToStream(stream types.Stream, message string, err error) error {
	log.Println("\n====[STREAM ERROR MESSAGE]====")
	if len(message) > 0 {
		log.Println(message)
	}
	if err != nil {
		log.Println(err)
	}
	log.Println("")

	return WriteMessageToStream(stream, BuildErrorMessage(message, err))
}

func WriteResponseToStream(stream types.Stream, response bool) error {
	return WriteMessageToStream(stream, BuildResponseMessage(response))
}

func WaitForResponse(stream types.Stream) (*types.ResponseMessage, error) {
	return ReadMessageFromStream[types.ResponseMessage](stream)
}

func WaitForUploadMessage(stream types.Stream) (*types.UploadMessage, error) {
	return ReadMessageFromStream[types.UploadMessage](stream)
}

func WaitForDownloadMessage(stream types.Stream) (*types.DownloadMessage, error) {
	return ReadMessageFromStream[types.DownloadMessage](stream)
}

func WaitForQueryMessage(stream types.Stream) (*types.QueryMessage, error) {
	return ReadMessageFromStream[types.QueryMessage](stream)
}

func WaitForAdvancedQueryMessage(stream types.Stream) (*types.AdvancedQueryMessage, error) {
	return ReadMessageFromStream[types.AdvancedQueryMessage](stream)
}

func ReadMessageFromStream[T any](stream types.Stream) (*T, error) {
	envelope, err := ReadEnvelopeFromStream(stream)
	if err != nil {
		return nil, err
	}

	if envelope.Type == "error" {
		errorBytes, err := cbor.Marshal(envelope.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error payload: %w", err)
		}

		var errorMsg types.ErrorMessage
		if err := cbor.Unmarshal(errorBytes, &errorMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error payload: %w", err)
		}

		log.Println("\n====[STREAM ERROR MESSAGE]====")
		if len(errorMsg.Message) > 0 {
			log.Println(errorMsg.Message)
		}
		log.Println("")

		return nil, fmt.Errorf("remote error: %s", errorMsg.Message)
	}

	payloadBytes, err := cbor.Marshal(envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var message T
	if err := cbor.Unmarshal(payloadBytes, &message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload to %T: %w", message, err)
	}

	return &message, nil
}

func ReadEnvelopeFromStream(stream types.Stream) (*types.MessageEnvelope, error) {
	streamDecoder := cbor.NewDecoder(stream)

	var envelope types.MessageEnvelope
	timeout := time.NewTimer(5 * time.Second)

wait:
	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("WaitForMessage timed out")
		default:
			err := streamDecoder.Decode(&envelope)

			if err != nil {
				return nil, err
			}

			if err == io.EOF {
				return nil, err
			}

			break wait
		}
	}

	return &envelope, nil
}

func WriteMessageToStream[T any](stream types.Stream, message T) error {
	typeName := reflect.TypeOf(message).String()

	envelope := types.MessageEnvelope{
		Type:    typeName,
		Payload: message,
	}

	enc := cbor.NewEncoder(stream)
	if err := enc.Encode(&envelope); err != nil {
		return err
	}

	return nil
}

func ReadJsonMessageFromStream[T any](stream types.Stream) (*T, error) {
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

func WriteJsonMessageToStream[T any](stream types.Stream, message T) error {
	enc := json.NewEncoder(stream)

	if err := enc.Encode(&message); err != nil {
		return err
	}

	return nil
}
