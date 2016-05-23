package sdk

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/Workiva/frugal/lib/go"
	"github.com/Workiva/go-vessel"
	"github.com/nats-io/nats"
	"golang.org/x/oauth2"
)

var (
	ErrIsOpen    = errors.New("frugal: client is already open")
	ErrNotOpen   = errors.New("frugal: client is not open")
	ErrNoServers = errors.New("frugal: no servers available for connection")
)

const minHeartbeatInterval = 500 * time.Millisecond

// ConnectionHandler is a callback invoked for different connection events.
type ConnectionHandler func()

// ErrorHandler is a callback invoked when there is an asynchronous error
// processing inbound messages.
type ErrorHandler func(err error)

// Client provides access to the messaging platform.
type Client struct {
	options Options
	conn    *nats.Conn
}

// New returns a new Client. Open must be called before the Client can be used.
func New(options Options) *Client {
	return &Client{options: options}
}

// SetClosedHandler sets the handler invoked when the client has been closed
// and will not attempt to reopen. The client must be manually reopened at this
// point.
func (c *Client) SetClosedHandler(handler ConnectionHandler) {
	c.conn.SetClosedHandler(func(_ *nats.Conn) {
		handler()
	})
}

// SetDisconnectHandler sets the handler invoked when the client has
// disconnected but might be attempting to reconnect. The closed handler will
// be invoked if reconnect fails. The reconnect handler will be invoked if
// reconnect succeeds.
func (c *Client) SetDisconnectHandler(handler ConnectionHandler) {
	c.conn.SetDisconnectHandler(func(_ *nats.Conn) {
		handler()
	})
}

// SetReconnectHandler sets the handler invoked when the client has
// successfully reconnected.
func (c *Client) SetReconnectHandler(handler ConnectionHandler) {
	c.conn.SetReconnectHandler(func(_ *nats.Conn) {
		handler()
	})
}

// SetErrorHandler sets the handler invoked when there is an asynchronous error
// processing inbound messages.
func (c *Client) SetErrorHandler(handler ErrorHandler) {
	c.conn.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		handler(err)
	})
}

// Open prepares the Client for use. Returns ErrIsOpen if the Client is
// already open and ErrNoServers if connecting to NATS times out.
func (c *Client) Open() error {
	if c.IsOpen() {
		return ErrIsOpen
	}

	// Chose a cluster representative to check for tls enabled
	clusterRep := c.options.NATSConfig.Url
	if len(c.options.NATSConfig.Servers) > 0 {
		clusterRep = c.options.NATSConfig.Servers[0]
	}

	// Set TLS credentials
	if strings.HasPrefix(clusterRep, "tls") {
		cert, err := tls.LoadX509KeyPair(c.options.ClientCertFilepath, c.options.ClientCertFilepath)
		if err != nil {
			return fmt.Errorf("messaging_sdk: Error loading client tls cert: %s", err)
		}

		cacert, err := ioutil.ReadFile(c.options.ClientCACertFilepath)
		if err != nil {
			return fmt.Errorf("messaging_sdk: Error loading client CA tls cert: %s", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cacert)

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
		}
		tlsConfig.BuildNameToCertificate()

		c.options.NATSConfig.TLSConfig = tlsConfig
	}
	conn, err := c.options.NATSConfig.Connect()
	if err != nil {
		if err == nats.ErrNoServers {
			return ErrNoServers
		}
		return err
	}
	c.conn = conn
	return nil
}

// IsOpen indicates if the Client has been opened for use.
func (c *Client) IsOpen() bool {
	return c.conn != nil && c.conn.Status() == nats.CONNECTED
}

// Close disconnects the Client.
func (c *Client) Close() error {
	if !c.IsOpen() {
		return ErrNotOpen
	}
	c.conn.Close()
	c.conn = nil
	return nil
}

// GetIamToken calls the IamTokenFetcher in the client options and returns a
// JWT token.
func (c *Client) GetIamToken() (string, error) {
	if c.options.IamTokenFetcher != nil {
		return c.options.IamTokenFetcher.GetIamToken()
	}
	return "", errors.New("messaging_sdk: No token fetcher present")
}

// Vessel returns a new Vessel client.
func (c *Client) Vessel() (vessel.Vessel, error) {
	if c.options.VesselAuth != nil {
		c.options.VesselConfig.HTTPClient = c.options.VesselAuth.Client(oauth2.NoContext)
	}
	return vessel.New(c.options.VesselHosts, &c.options.VesselConfig)
}

// ProvideClient returns the plumbing required for a Frugal client of the
// given service. You must explicitly open the transport before using it with
// a Frugal client.
func (c *Client) ProvideClient(service Service) (frugal.FTransport, *frugal.FProtocolFactory, error) {
	if !c.IsOpen() {
		return nil, nil, ErrNotOpen
	}

	tProtocolFactory, err := c.options.newThriftProtocolFactory()
	if err != nil {
		return nil, nil, err
	}
	fProtocolFactory := frugal.NewFProtocolFactory(tProtocolFactory)
	transportFactory := c.options.newThriftTransportFactory()
	transport := frugal.NewNatsServiceTTransport(c.conn, string(service),
		c.options.ConnectTimeout, c.options.MaxMissedHeartbeats)
	tr := transportFactory.GetTransport(transport)
	fTransport := frugal.NewFMuxTransport(tr, c.options.NumWorkers)
	return fTransport, fProtocolFactory, nil
}

// ProvideServer returns a Frugal server for the given service.
func (c *Client) ProvideServer(service Service, processor frugal.FProcessor) (frugal.FServer, error) {
	if !c.IsOpen() {
		return nil, ErrNotOpen
	}

	tProtocolFactory, err := c.options.newThriftProtocolFactory()
	if err != nil {
		return nil, err
	}
	fProtocolFactory := frugal.NewFProtocolFactory(tProtocolFactory)
	fTransportFactory := frugal.NewFMuxTransportFactory(c.options.NumWorkers)
	heartbeatInterval := c.options.HeartbeatInterval
	if heartbeatInterval < minHeartbeatInterval {
		heartbeatInterval = minHeartbeatInterval
	}
	return frugal.NewFNatsServerFactory(c.conn, string(service),
		heartbeatInterval, c.options.MaxMissedHeartbeats,
		frugal.NewFProcessorFactory(processor), fTransportFactory, fProtocolFactory), nil
}

// ProvidePubSub returns the plumbing required for Frugal pub/sub.
func (c *Client) ProvidePubSub() (*frugal.FScopeProvider, error) {
	protocolFactory, err := c.options.newThriftProtocolFactory()
	if err != nil {
		return nil, err
	}

	natsFactory := frugal.NewFNatsScopeTransportFactory(c.conn)
	return frugal.NewFScopeProvider(natsFactory, frugal.NewFProtocolFactory(protocolFactory)), nil
}
