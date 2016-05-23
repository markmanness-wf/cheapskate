package vessel

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

const (
	// TCP is the name of the raw TCP socket transport.
	TCP Transport = "tcp"
)

// tcpConn implements the conn interface using tcp as the underlying
// transport layer.
type tcpConn struct {
	conn     net.Conn
	socketMu sync.Mutex
}

// Close the connection
func (t *tcpConn) Close() error {
	return t.conn.Close()
}

// Send data via the connection
func (t *tcpConn) Send(data []byte, msgType proto) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, msgType); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(data))); err != nil {
		return err
	}
	if _, err := buf.Write(data); err != nil {
		return err
	}

	t.socketMu.Lock()
	_, err := t.conn.Write(buf.Bytes())
	t.socketMu.Unlock()
	return err
}

// Receive data from the connection
func (t *tcpConn) Recv() ([]byte, error) {
	var size uint64
	if err := binary.Read(t.conn, binary.BigEndian, &size); err != nil {
		return nil, err
	}

	data := make([]byte, size)
	_, err := io.ReadFull(t.conn, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tcpTransport implements the Transport interface using raw TCP sockets as the
// underlying transport layer.
type tcpTransport struct {
	*connectionTransport
}

// newTCPTransport creates and returns a new Transport which communicates
// with a vessel using TCP sockets.
func newTCPTransport(config *Config) (transport, error) {
	ct, err := newConnectionTransport(config)
	if err != nil {
		return nil, err
	}

	transport := &tcpTransport{
		connectionTransport: ct,
	}

	return transport, nil
}

// Dial resolves the vessel at the provided host. It returns an error if the
// vessel is not available, the transport is not supported, or a session could
// not be created.
func (t *tcpTransport) Dial(host string) error {
	if err := t.dial(host); err != nil {
		return err
	}

	addr, err := t.resolveTCP(host)
	if err != nil {
		return err
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		t.Close()
		return err
	}

	if t.tls != nil {
		if t.conn, err = tcpTLSDial("tcp", tcpAddr, t.tls); err != nil {
			t.Close()
			return err
		}
	} else {
		if t.conn, err = tcpDial("tcp", nil, tcpAddr); err != nil {
			t.Close()
			return err
		}
	}

	if err := t.handshake(); err != nil {
		t.Close()
		return err
	}
	if t.listen {
		go t.dispatchLoop()
		go t.recvLoop()
		if t.config.batching() {
			go t.sendLoop()
		}
		if t.config.Heartbeat {
			go t.heartbeat()
		}
	}

	return nil
}

var tcpDial = func(n string, laddr, raddr *net.TCPAddr) (conn, error) {
	conn, err := net.DialTCP(n, laddr, raddr)
	return &tcpConn{conn: conn}, err
}

var tcpTLSDial = func(n string, addr *net.TCPAddr, config *tls.Config) (conn, error) {
	conn, err := tls.Dial(n, addr.String(), config)
	return &tcpConn{conn: conn}, err
}

func (t *tcpTransport) resolveTCP(addr string) (string, error) {
	meta, err := t.resolveTransport(addr, TCP)
	if err != nil {
		return "", err
	}

	port, ok := meta["port"]
	if !ok {
		return "", fmt.Errorf("Could not resolve transport: %s", addr)
	}

	return getTCPAddr(addr, port.(string)), nil
}

func getTCPAddr(host, port string) string {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	return strings.Split(host, ":")[0] + ":" + port
}
