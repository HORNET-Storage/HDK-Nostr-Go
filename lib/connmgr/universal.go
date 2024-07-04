package connmgr

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func SendUniversalEvent(ctx context.Context, connectionManager ConnectionManager, event *nostr.Event, subscriptionId *string) (map[string]*nostr.OKEnvelope, error) {
	results := map[string]*nostr.OKEnvelope{}

	for connectionID := range connectionManager.ListConnections() {
		env, err := SendUniversalEventSingle(ctx, connectionManager, connectionID, event, subscriptionId)

		if env != nil {
			results[connectionID] = env
		}

		if err != nil {
			return nil, fmt.Errorf("failed to upload DAG to node %s: %w", connectionID, err)
		}
	}

	return results, nil
}

func SendUniversalEventSingle(ctx context.Context, connectionManager ConnectionManager, connectionID string, event *nostr.Event, subscriptionId *string) (*nostr.OKEnvelope, error) {
	stream, err := connectionManager.GetStream(ctx, connectionID, "/nostr/event/universal")
	if err != nil {
		return nil, fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	streamEncoder := json.NewEncoder(stream)

	reqMessage := nostr.EventEnvelope{
		SubscriptionID: subscriptionId,
		Event:          *event,
	}

	if err := streamEncoder.Encode(&reqMessage); err != nil {
		return nil, err
	}

	response, err := ReadJsonMessageFromStream[[]json.RawMessage](stream)
	if err != nil {
		return nil, err
	}

	message := *response

	if len(message) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	var typ string
	if err = json.Unmarshal(message[0], &typ); err != nil {
		return nil, err
	}

	switch typ {
	case "NOTICE":
		if len(message) != 2 {
			return nil, fmt.Errorf("invalid notice message length: %d", len(message))
		}

		var m string
		if err = json.Unmarshal(message[1], &m); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf(m)
	case "OK":
		if len(message) != 4 {
			return nil, fmt.Errorf("invalid OK message length: %d", len(message))
		}

		var eventID string
		if err = json.Unmarshal(message[1], &eventID); err != nil {
			return nil, err
		}
		var ok bool
		if err = json.Unmarshal(message[2], &ok); err != nil {
			return nil, err
		}
		var m string
		if err = json.Unmarshal(message[3], &m); err != nil {
			return nil, err
		}

		if !ok {
			return nil, fmt.Errorf(m)
		}

		okEnv := &nostr.OKEnvelope{
			EventID: eventID,
			OK:      ok,
			Reason:  m,
		}

		return okEnv, nil
	}

	return nil, fmt.Errorf("invalid response")
}

func QueryEvents(ctx context.Context, connectionManager ConnectionManager, connectionID string, filters []nostr.Filter, subscriptionId *string) ([]nostr.Event, error) {
	stream, err := connectionManager.GetStream(ctx, connectionID, "/nostr/event/universal")
	if err != nil {
		return nil, fmt.Errorf("failed to get stream for connection %s: %w", connectionID, err)
	}
	defer stream.Close()

	streamEncoder := json.NewEncoder(stream)

	reqMessage := nostr.ReqEnvelope{
		Filters: filters,
	}

	if subscriptionId != nil {
		reqMessage.SubscriptionID = *subscriptionId
	}

	if err := streamEncoder.Encode(&reqMessage); err != nil {
		return nil, err
	}

	events := []nostr.Event{}
	eose := false

	for {
		response, err := ReadJsonMessageFromStream[[]json.RawMessage](stream)
		if err != nil {
			return nil, err
		}

		message := *response

		if len(message) == 0 {
			return nil, fmt.Errorf("empty message")
		}
		var typ string
		if err = json.Unmarshal(message[0], &typ); err != nil {
			return nil, err
		}

		switch typ {
		case "NOTICE":
			if len(message) != 2 {
				return nil, fmt.Errorf("invalid notice message length: %d", len(message))
			}

			var m string
			if err = json.Unmarshal(message[1], &m); err != nil {
				return nil, err
			}

			return nil, fmt.Errorf(m)
		case "EVENT":
			if len(message) != 3 {
				return nil, fmt.Errorf("invalid event message length: %d", len(message))
			}

			var subID string
			if err = json.Unmarshal(message[1], &subID); err != nil {
				return nil, err
			}

			// Temporary check until subscriptions are fully supported
			if subscriptionId != nil && subID != *subscriptionId {
				return nil, fmt.Errorf("wrong subscription id")
			}

			var eventStr string
			if err = json.Unmarshal(message[2], &eventStr); err != nil {
				return nil, err
			}

			var event nostr.Event
			if err = json.Unmarshal([]byte(eventStr), &event); err != nil {
				return nil, err
			}

			events = append(events, event)
		case "EOSE":
			eose = true

			stream.Close()
		}

		if eose {
			break
		}
	}

	return events, nil
}
