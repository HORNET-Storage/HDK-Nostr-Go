package websocket

import (
	"bytes"
	"context"
	"io"

	"github.com/gorilla/websocket"
)

type WebSocketStream struct {
	conn        *websocket.Conn
	ctx         context.Context
	writeBuffer bytes.Buffer
}

func NewWebSocketStream(conn *websocket.Conn, ctx context.Context) *WebSocketStream {
	return &WebSocketStream{
		conn: conn,
		ctx:  ctx,
	}
}

func (ws *WebSocketStream) Read(msg []byte) (int, error) {
	_, reader, err := ws.conn.NextReader()
	if err != nil {
		return 0, err
	}
	return io.ReadFull(reader, msg)
}

func (ws *WebSocketStream) Write(msg []byte) (int, error) {
	ws.writeBuffer.Write(msg)
	return len(msg), nil
}

func (ws *WebSocketStream) Flush() error {
	w, err := ws.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}
	if _, err := w.Write(ws.writeBuffer.Bytes()); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	ws.writeBuffer.Reset()
	return nil
}

func (ws *WebSocketStream) Close() error {
	return ws.conn.Close()
}

func (ws *WebSocketStream) Context() context.Context {
	return ws.ctx
}
