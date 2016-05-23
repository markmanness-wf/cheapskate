package vessel

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/websocket"
)

const (
	// WS is the name of the WebSocket transport.
	WS Transport = "ws"
)

// websocketConn implements the conn interface using websockets as the
// underlying transport.
type websocketConn struct {
	conn *websocket.Conn
}

// Close the connection
func (ws *websocketConn) Close() error {
	if ws.conn != nil {
		return ws.conn.Close()
	}
	return nil
}

// Send data via the connection
func (ws *websocketConn) Send(data []byte, _ proto) error {
	return websocket.Message.Send(ws.conn, data)
}

// Receive data from the connection
func (ws *websocketConn) Recv() ([]byte, error) {
	var data []byte
	err := websocket.Message.Receive(ws.conn, &data)
	return data, err
}

// websocketTransport implements the Transport interface using WebSockets as the
// underlying transport layer.
type websocketTransport struct {
	*connectionTransport
}

// newWSTransport creates and returns a new Transport which communicates
// with a vessel using WebSockets.
func newWSTransport(config *Config) (transport, error) {
	ct, err := newConnectionTransport(config)
	if err != nil {
		return nil, err
	}

	transport := &websocketTransport{
		connectionTransport: ct,
	}

	return transport, nil
}

// Dial resolves the vessel at the provided host. It returns an error if
// the vessel is not available, the transport is not supported, or a
// session could not be created.
func (ws *websocketTransport) Dial(host string) error {
	if err := ws.dial(host); err != nil {
		return err
	}

	config, err := ws.resolveAddress(host)
	if err != nil {
		return err
	}

	ws.conn, err = dialWS(config)
	if err != nil {
		ws.Close()
		return err
	}

	if err := ws.handshake(); err != nil {
		ws.Close()
		return err
	}

	if ws.listen {
		go ws.dispatchLoop()
		go ws.recvLoop()
		if ws.config.batching() {
			go ws.sendLoop()
		}
		if ws.config.Heartbeat {
			go ws.heartbeat()
		}
	}

	return nil
}

// Dial a websocket connection, abstracts standard library function
// to allow for unit testing
var dialWS = func(config *websocket.Config) (conn, error) {
	conn, err := websocket.DialConfig(config)
	return &websocketConn{conn: conn}, err
}

// Generates a websocket config struct based on an address
func (ws *websocketTransport) resolveAddress(address string) (*websocket.Config, error) {
	meta, err := ws.resolveTransport(address, WS)
	if err != nil {
		return nil, err
	}

	port, ok := meta["port"]
	if !ok {
		return nil, fmt.Errorf("Could not resolve transport: %s", address)
	}

	uri, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	host := strings.Split(uri.Host, ":")[0]
	origin := uri.Scheme + "://" + host

	if !ws.config.Proxy {
		uri.Host = host + ":" + port.(string)
	}
	if uri.Scheme == "https" {
		uri.Scheme = "wss"
	} else {
		uri.Scheme = "ws"
	}

	config, err := websocket.NewConfig(uri.String(), origin)
	if err != nil {
		return nil, err
	}
	config.TlsConfig = ws.tls

	return config, nil
}
