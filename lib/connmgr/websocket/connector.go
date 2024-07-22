package websocket

import (
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
)

type WebSocketConnector struct {
	URL  string
	Conn *websocket.Conn
}

func NewWebSocketConnector(url string) *WebSocketConnector {
	return &WebSocketConnector{URL: fmt.Sprintf("%s/scionic", url)}
}

func (wsc *WebSocketConnector) Connect(ctx context.Context) error {
	log.Panicln("This does not need to be called for websockets, just use OpenStream")
	return nil
}

func (wsc *WebSocketConnector) Disconnect() error {
	return wsc.Conn.Close()
}

func (wsc *WebSocketConnector) OpenStream(ctx context.Context, protocolID string) (types.Stream, error) {
	var d websocket.Dialer
	conn, _, err := d.DialContext(ctx, fmt.Sprintf("%s/%s", wsc.URL, protocolID), nil)
	if err != nil {
		return nil, err
	}
	wsc.Conn = conn

	return &WebSocketStream{conn: wsc.Conn, ctx: ctx}, nil
}
