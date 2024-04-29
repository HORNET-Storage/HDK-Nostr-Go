package connmgr

import (
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

type Client struct {
	serverAddress *string
	publicKey     *string

	Host          *host.Host
	ServerAddress *multiaddr.Multiaddr
	Peer          *peer.AddrInfo
}

const (
	UploadV1   protocol.ID = "/upload/1.0.0"
	DownloadV1 protocol.ID = "/download/1.0.0"
)

var Clients map[string]*Client

func init() {
	Clients = map[string]*Client{}
}

func Connect(ctx context.Context, serverAddress string, publicKey string, opts ...config.Option) (context.Context, *Client, error) {
	host, err := libp2p.New(opts...)
	if err != nil {
		return ctx, nil, err
	}

	//serverAddress := fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", ip, port, publicKey)
	maddr, err := multiaddr.NewMultiaddr(serverAddress)
	if err != nil {
		return ctx, nil, err
	}
	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return ctx, nil, err
	}
	if err := host.Connect(ctx, *serverInfo); err != nil {
		return ctx, nil, err
	}

	client := &Client{
		serverAddress: &serverAddress,
		publicKey:     &publicKey,

		Host:          &host,
		ServerAddress: &maddr,
		Peer:          serverInfo,
	}

	Clients[publicKey] = client

	log.Println("Connected to:", serverInfo)

	return ctx, client, nil
}

func Disconnect(publicKey string) error {
	client, err := GetClient(publicKey)
	if err != nil {
		return err
	}

	delete(Clients, publicKey)

	host := *client.Host

	host.Close()

	return nil
}

func GetClient(publicKey string) (*Client, error) {
	client, exists := Clients[publicKey]

	if !exists {
		return nil, fmt.Errorf("host for this public key does not exist")
	}

	return client, nil
}

func (client *Client) Disconnect() error {
	delete(Clients, *client.publicKey)

	host := *client.Host

	host.Close()

	return nil
}

func (client *Client) openStream(ctx context.Context, protocol protocol.ID) (context.Context, network.Stream, error) {
	host := *client.Host

	stream, err := host.NewStream(ctx, client.Peer.ID, protocol)
	if err != nil {
		return ctx, nil, err
	}

	return ctx, stream, nil
}
