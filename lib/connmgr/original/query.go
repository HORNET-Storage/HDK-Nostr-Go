package original

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func (client *Client) QueryEvents(ctx context.Context, filters []nostr.Filter, subscriptionId *string) (context.Context, []nostr.Event, error) {
	ctx, stream, err := client.openStream(ctx, "/nostr/event/filter")
	if err != nil {
		return ctx, nil, err
	}

	streamEncoder := json.NewEncoder(stream)

	reqMessage := nostr.ReqEnvelope{
		Filters: filters,
	}

	if subscriptionId != nil {
		reqMessage.SubscriptionID = *subscriptionId
	}

	if err := streamEncoder.Encode(&reqMessage); err != nil {
		return ctx, nil, err
	}

	events := []nostr.Event{}
	eose := false

	for {
		response, err := ReadJsonMessageFromStream[[]json.RawMessage](stream)
		if err != nil {
			return ctx, nil, err
		}

		message := *response

		if len(message) == 0 {
			return ctx, nil, fmt.Errorf("empty message")
		}
		var typ string
		if err = json.Unmarshal(message[0], &typ); err != nil {
			return ctx, nil, err
		}

		switch typ {
		case "NOTICE":
			if len(message) != 2 {
				return ctx, nil, fmt.Errorf("invalid notice message length: %d", len(message))
			}

			var m string
			if err = json.Unmarshal(message[1], &m); err != nil {
				return ctx, nil, err
			}

			return ctx, nil, fmt.Errorf(m)
		case "EVENT":
			if len(message) != 3 {
				return ctx, nil, fmt.Errorf("invalid event message length: %d", len(message))
			}

			var subID string
			if err = json.Unmarshal(message[1], &subID); err != nil {
				return ctx, nil, err
			}

			// Temporary check until subscriptions are fully supported
			if subscriptionId != nil && subID != *subscriptionId {
				return ctx, nil, fmt.Errorf("wrong subscription id")
			}

			var eventStr string
			if err = json.Unmarshal(message[2], &eventStr); err != nil {
				return ctx, nil, err
			}

			var event nostr.Event
			if err = json.Unmarshal([]byte(eventStr), &event); err != nil {
				return ctx, nil, err
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

	return ctx, events, nil
}
