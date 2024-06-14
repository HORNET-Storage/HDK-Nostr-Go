package websocket

import (
	"context"

	"github.com/gorilla/websocket"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
)

type WebSocketConnector struct {
	URL  string
	Conn *websocket.Conn
}

func NewWebSocketConnector(url string) *WebSocketConnector {
	return &WebSocketConnector{URL: url}
}

func (wsc *WebSocketConnector) Connect(ctx context.Context) error {
	var d websocket.Dialer
	conn, _, err := d.DialContext(ctx, wsc.URL, nil)
	if err != nil {
		return err
	}
	wsc.Conn = conn
	return nil
}

func (wsc *WebSocketConnector) Disconnect() error {
	return wsc.Conn.Close()
}

func (wsc *WebSocketConnector) OpenStream(ctx context.Context, protocolID string) (types.Stream, error) {
	return &WebSocketStream{conn: wsc.Conn, ctx: ctx}, nil
}
