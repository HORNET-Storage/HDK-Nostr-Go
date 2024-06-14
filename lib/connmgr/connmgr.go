package connmgr

import (
	"context"
	"fmt"
	"sync"

	types "github.com/HORNET-Storage/go-hornet-storage-lib/lib"
	"github.com/libp2p/go-libp2p"

	libp2pConnector "github.com/HORNET-Storage/go-hornet-storage-lib/lib/connmgr/libp2p"
	websocketConnector "github.com/HORNET-Storage/go-hornet-storage-lib/lib/connmgr/websocket"
)

const (
	UploadV1   string = "/upload/1.0.0"
	DownloadV1 string = "/download/1.0.0"
	QueryV1    string = "/query/1.0.0"
)

type ConnectionManager interface {
	ConnectWithLibp2p(ctx context.Context, connectionId string, serverAddress string, opts ...libp2p.Option) error
	ConnectWithWebsocket(ctx context.Context, connectionId string, url string) error
	Disconnect(connectionID string) error
	GetStream(ctx context.Context, connectionID string, protocolID string) (types.Stream, error)
	ListConnections() map[string]types.Connector
}

type GenericConnectionManager struct {
	connections map[string]types.Connector
	mutex       sync.RWMutex
}

func SetupConnection(useLibp2p bool, address string) (types.Connector, error) {
	if useLibp2p {
		return libp2pConnector.NewLibp2pConnector(address, libp2p.Defaults)
	} else {
		return websocketConnector.NewWebSocketConnector(address), nil
	}
}

func NewGenericConnectionManager() *GenericConnectionManager {
	return &GenericConnectionManager{
		connections: make(map[string]types.Connector),
	}
}

func (gcm *GenericConnectionManager) ListConnections() map[string]types.Connector {
	return gcm.connections
}

func (gcm *GenericConnectionManager) ConnectWithLibp2p(ctx context.Context, connectionId string, serverAddress string, opts ...libp2p.Option) error {
	gcm.mutex.Lock()
	defer gcm.mutex.Unlock()

	connector, err := libp2pConnector.NewLibp2pConnector(serverAddress, opts...)

	if err != nil {
		return err
	}

	err = connector.Connect(ctx)
	if err != nil {
		return err
	}

	gcm.connections[connectionId] = connector
	return nil
}

func (gcm *GenericConnectionManager) ConnectWithWebsocket(ctx context.Context, connectionId string, url string) error {
	gcm.mutex.Lock()
	defer gcm.mutex.Unlock()

	connector := websocketConnector.NewWebSocketConnector(url)

	err := connector.Connect(ctx)
	if err != nil {
		return err
	}

	gcm.connections[connectionId] = connector
	return nil
}

func (gcm *GenericConnectionManager) Disconnect(connectionID string) error {
	gcm.mutex.Lock()
	defer gcm.mutex.Unlock()

	if connector, exists := gcm.connections[connectionID]; exists {
		err := connector.Disconnect()
		delete(gcm.connections, connectionID)
		return err
	}
	return nil
}

func (gcm *GenericConnectionManager) GetStream(ctx context.Context, connectionID string, protocolID string) (types.Stream, error) {
	gcm.mutex.RLock()
	defer gcm.mutex.RUnlock()

	if connector, exists := gcm.connections[connectionID]; exists {
		return connector.OpenStream(ctx, protocolID)
	}
	return nil, fmt.Errorf("no connection found with ID %s", connectionID)
}
