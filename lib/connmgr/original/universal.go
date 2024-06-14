package original

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func (client *Client) SendUniversalEvent(ctx context.Context, event *nostr.Event, subscriptionId *string) (context.Context, *nostr.OKEnvelope, error) {
	ctx, stream, err := client.openStream(ctx, "/nostr/event/universal")
	if err != nil {
		return ctx, nil, err
	}

	streamEncoder := json.NewEncoder(stream)

	reqMessage := nostr.EventEnvelope{
		SubscriptionID: subscriptionId,
		Event:          *event,
	}

	if err := streamEncoder.Encode(&reqMessage); err != nil {
		return ctx, nil, err
	}

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
	case "OK":
		if len(message) != 4 {
			return ctx, nil, fmt.Errorf("invalid OK message length: %d", len(message))
		}

		var eventID string
		if err = json.Unmarshal(message[1], &eventID); err != nil {
			return ctx, nil, err
		}
		var ok bool
		if err = json.Unmarshal(message[2], &ok); err != nil {
			return ctx, nil, err
		}
		var m string
		if err = json.Unmarshal(message[3], &m); err != nil {
			return ctx, nil, err
		}

		if !ok {
			return ctx, nil, fmt.Errorf(m)
		}

		okEnv := &nostr.OKEnvelope{
			EventID: eventID,
			OK:      ok,
			Reason:  m,
		}

		return ctx, okEnv, nil
	}

	return ctx, nil, fmt.Errorf("invalid response")
}
