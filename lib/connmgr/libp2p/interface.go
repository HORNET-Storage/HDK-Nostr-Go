package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
)

type Libp2pStream struct {
	stream network.Stream
	ctx    context.Context
}

func (ls *Libp2pStream) Read(msg []byte) (int, error) {
	return ls.stream.Read(msg)
}

func (ls *Libp2pStream) Write(msg []byte) (int, error) {
	return ls.stream.Write(msg)
}

func (ls *Libp2pStream) Close() error {
	return ls.stream.Close()
}

func (ls *Libp2pStream) Context() context.Context {
	return ls.ctx
}
