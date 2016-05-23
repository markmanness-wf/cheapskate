package vessel

import (
	"errors"
	"net/http"
	"time"

	"github.com/Workiva/go-datastructures/batcher"
)

// Transport is the name of a vessel transport.
type Transport string

// ContentType is the encoding that should be used to send an envelope to the server.
type ContentType string

const (
	// JSONContent represents using JSON to serialize an envelope
	JSONContent ContentType = "application/json"

	// MsgpackConent represents using msgpack to serialize an envlope
	MsgpackContent ContentType = "application/msgpack"

	defaultReconnectBackoff = 500 * time.Millisecond

	defaultBatchMaxTime          = 5 * time.Millisecond
	defaultBatchMaxBytes    uint = 0
	defaultBatchMaxMessages uint = 20
	defaultBatchQueueLen         = 5
)

// ErrAckTimeout is returned when a message does not receive an ack before its
// deadline and all retries have been exceeded.
var ErrAckTimeout = errors.New("Timed out before receiving ack")

// Option is a configurable Transport parameter. A Transport will ignore
// Options that it doesn't use.
type Option string

// Options specify parameters for outgoing messages.
type Options struct {
	QoS          QoS
	Retries      uint
	RetryBackoff bool
	AckTimeout   time.Duration
	TTL          time.Duration
	Header       http.Header
}

// Config contains configuration parameters for a Vessel client.
// Setting all of the batching parameters to zero will disable batching
// and revert to the old behavior of sending messages synchronously as soon
// as requested. A batch is determined "ready" when a single condition is met.
// So for a BatchMaxEnvelopes of 20 and a BatchMaxTime of 200ms, if 20 items are
// sent in 50ms, a batch will be formed and sent without waiting another 150ms.
// Config options cannot be modified once the client has started or a race
// condition may occur.
type Config struct {
	// ClientID acts as an address for other clients to send messages to this
	// client.
	ClientID string

	// HTTPClient is used to make HTTP requests to the Vessel server, including
	// establishing and renewing sessions. A client can be injected to provide
	// authentication.
	HTTPClient *http.Client

	// Fallbacks are the Transports to attempt to connect with in priority
	// order.
	Fallbacks []Transport

	// Options contain Transport options. If an Option doesn't apply to a
	// Transport, it will be ignored.
	Options map[Option]interface{}

	// AllowEchoes determines if the client should receive messages which it
	// sent.
	AllowEchoes bool

	// ReconnectRetries controls how many times to attempt to reconnect if the
	// connection to the Vessel server is dropped.
	ReconnectRetries uint

	// ReconnectBackoff indicates if the client should exponentially backoff
	// when attempting to reconnect.
	ReconnectBackoff bool

	// ReconnectDelay controls how long to wait between reconnect attempts.
	ReconnectDelay time.Duration

	// ContentType determines what content type to use to communicate with the
	// Vessel server.
	ContentType ContentType

	// ClientBufferSize is the size of the client message buffer. Defaults to
	// 100k.
	ClientBufferSize uint

	// InternalBufferSize is the size of the internal buffer used by
	// transports. Defaults to 200k.
	InternalBufferSize uint

	// BatchMaxTime is the maximum amount of time a message should wait to be
	// put in a batch. Set to zero to ignore. Defaults to 5 milliseconds.
	BatchMaxTime time.Duration

	// BatchMaxBytes is the number of bytes a batch should be allowed to build
	// up. Set to zero to ignore. Ignored by default.
	BatchMaxBytes uint

	// BatchMaxMessages is the number of envelopes a batch should be allowed to
	// have. Set to zero to ignore. Defaults to 20 envelopes.
	BatchMaxMessages uint

	// BatchQueueLen is the maximum number of batches that should be allowed to
	// sit in a queue ready to send. This blocks on put if the queue is full,
	// similar to a channel, so a larger queue length will give better throughput.
	BatchQueueLen uint

	// BatchCalcBytes determines how to evaluate how many bytes are in a
	// message, defaults to the size of the message body.
	BatchCalcBytes batcher.CalculateBytes

	// Proxy determines if the client should ignore the internal addresses
	// returned by Vessel.
	Proxy bool

	// Filters are used to multiplex messages into channels. Each message
	// received by the client passes through each filter before reaching the
	// messages or subscription channel. If the message matches the filter
	// predicate, a copy of it is placed on the channel. If the filter filters
	// the message, it won't be propagated further. Filters are applied in the
	// order they are provided.
	Filters []Filter

	// Heartbeat determines if the client should periodically ping the vessel server,
	// ensuring a persistent connection is not terminated prematurely.
	// This is useful if the client is using a proxy.
	Heartbeat bool

	// HeartbeatInterval is the number of seconds between each "ping" and resulting "pong"
	// from the vessel server. Defaults to 15 seconds.
	HeartbeatInterval time.Duration

	// Header is an optional HTTP header for Session creation and renewal.
	Header http.Header

	closed       chan bool
	events       chan Event
	disconnected chan bool
}

func (c *Config) batching() bool {
	return c.BatchMaxTime != 0 || c.BatchMaxBytes != 0 || c.BatchMaxMessages != 0
}

// dropEcho indicates if the envelope is an echo, meaning it was sent by this
// client, and if it should be dropped.
func (c *Config) dropEcho(env *Envelope) bool {
	return c.ClientID == env.origin() && !c.AllowEchoes
}

// connect signals that the connection has been established.
func (c *Config) connect() {
	c.emitEvent(Event{Type: EventConnected})
}

// reconnectFailed signals that reconnection has failed.
func (c *Config) reconnectFailed() {
	c.emitEvent(Event{Type: EventReconnectFailed})
}

// disconnect signals that the connection has been disconnected.
func (c *Config) disconnect() {
	c.emitEvent(Event{Type: EventDisconnected})

	// TODO: remove when closed channel is removed.
	select {
	case c.closed <- true:
	default:
	}
}

// emitEvent sends the given event on the events channel. If the channel is
// full, the event is dropped.
func (c *Config) emitEvent(event Event) {
	event.Timestamp = time.Now()
	select {
	case c.events <- event:
	default:
	}
}

func (c *Config) drainDisconnect() {
	select {
	case <-c.closed:
	default:
	}
}

// applyFilters applies the Config Filters and returns a bool indicating if
// the Message should not be propagated.
func (c *Config) applyFilters(message *Message) bool {
	for _, filter := range c.Filters {
		if filter.apply(message) {
			return true
		}
	}
	return false
}

// NewConfig creates a new default Config. If Transports are specified, they
// will be used in the order they are provided until the first one is resolved
// on Dial. If no Transports are specified, TCP will be used with HTTP as a
// fallback.
func NewConfig(clientID string, transports ...Transport) *Config {
	return &Config{
		ClientID:         clientID,
		HTTPClient:       http.DefaultClient,
		Fallbacks:        transports,
		ReconnectDelay:   defaultReconnectBackoff,
		ReconnectBackoff: true,
		ReconnectRetries: 5,
		ContentType:      MsgpackContent,
		BatchMaxTime:     defaultBatchMaxTime,
		BatchMaxBytes:    defaultBatchMaxBytes,
		BatchMaxMessages: defaultBatchMaxMessages,
		BatchQueueLen:    defaultBatchQueueLen,
		BatchCalcBytes:   envelopeBodySize,
	}
}
