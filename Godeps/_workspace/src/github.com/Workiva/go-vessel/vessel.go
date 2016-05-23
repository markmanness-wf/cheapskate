/*
Package vessel provides a client API for communicating with a Vessel broker.
*/
package vessel

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"time"
)

// These constants are used by the client to identify to Vessel the library
// version being used.
const (
	clientName    = "go-vessel"
	clientVersion = "2.2.3"
)

// getClientVersion returns the formatted version string used to identify
// the version to Vessel.
func getClientVersion() string {
	return fmt.Sprintf("%s/%s", clientName, clientVersion)
}

const (
	// FireAndForget means there is no delivery guarantee for a message.
	// Messages are fast but may not be received.
	FireAndForget QoS = 0

	// AtLeastOnce means there is an at-least-once delivery guarantee for a
	// message. Messages are slow because they are durably persisted and acked.
	AtLeastOnce QoS = 1

	defaultAckTimeout = 3 * time.Second
	eventsQLen        = 10
)

var (
	// ErrNotConnected is returned when an operation is performed on a Vessel
	// client which is not connected.
	ErrNotConnected = errors.New("Transport not connected")

	// batchVersion is the minimum version of the server that can support
	// batching. This should be checked for backward compatibility.
	batchVersion = version{Major: 2, Minor: 1, Patch: 0}
)

// Subscription contains parameters for a channel subscription used with
// Subscribe and Unsubscribe.
type Subscription struct {
	// Vessel channel name. Vessel supports pattern matching for channels.
	// See https://github.com/Workiva/matchbox#wildcards for details.
	Channel string
	// Acknowledgement timeout for subscription request acknowledgement.
	// Defaults to 5 seconds.
	Timeout time.Duration
	// Size for the Message buffer as returned on call to Subscribe. Defaults
	// to 2^10.
	BufferSize uint
	id         string
}

// EventType describes a Vessel event.
type EventType byte

// EventType constants which describe Vessel connection events.
const (
	// EventSessionExpired indicates the current session has expired.
	EventSessionExpired EventType = iota
	// EventRateLimit indicates you are currently being rate limited
	// for sending too many messages
	EventRateLimit
	// EventOriginMismatch indicates a message sent with your session
	// didn't have the correct origin ID and wasn't processed.
	EventOriginMismatch
	// EventBackPressure indicates a dropped message because the Vessel
	// server is currently under a high load.
	EventBackpressure
	// EventReadbackFinished indicates all of the messages missed in a
	// connection disconnect/reconnect scenario have been received.
	EventReadbackFinished

	// EventReconnectFailed indicates the client has given up on reconnecting
	// with the Vessel server
	EventReconnectFailed EventType = 0xFC
	// EventSessionRenewed indicates the current session has been renewed.
	EventSessionRenewed EventType = 0xFD
	// EventConnected indicates the client has successfully connected with
	// a Vessel server.
	EventConnected EventType = 0xFE
	// EventDisconnected indicates the client has become disconnected from
	// the Vessel server it had connected to.
	EventDisconnected EventType = 0xFF
)

// Event is a notification to the client resulting from a change in connection
// state.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      []byte
}

// Vessel is the top-level API for communicating with a vessel using an
// underlying transport.
type Vessel interface {
	// Dial resolves the vessel at the provided host. It returns an error if
	// the vessel is not available, the transport is not supported, or a
	// session could not be created.
	Dial() error

	// SessionID returns the ID of the currently connected Session. If the
	// Vessel has not been dialed, the behavior is undefined.
	SessionID() string

	// AttemptDial makes successive calls to Dial until the dial limit is reached
	// or a successful dial occurs. A backoff timeout used to wait between each
	// dial attempt.
	AttemptDial(dialLimit int64, backoff time.Duration) error

	// Close terminates the vessel session and performs a graceful shutdown of
	// the transport client.
	Close() error

	// Subscribe returns a chan which yields messages published on the
	// specified vessel Subscriptions. It returns an error if the subscription
	// fails.
	Subscribe(subscription *Subscription) (chan *Message, error)

	// AddSubscriptionsToChannel adds subscriptions to the passed in Go channel.
	AddSubscriptionsToChannel(messageChannel chan *Message, subscriptions ...*Subscription) error

	// Unsubscribe closes the chan returned by Subscribe and unsubscribes the
	// client from the channel. It returns an error if the client wasn't
	// subscribed to the channel.
	Unsubscribe(subscription *Subscription) error

	// Publish broadcasts the message on the specified channel. It returns two
	// channels, one which returns the id of the published message and one
	// which returns any errors which occur. If the message requires an ack and
	// a timeout occurs before an ack is received and retries are exceeded,
	// ErrAckTimeout is returned on this channel.
	Publish(payload []byte, subscription *Subscription, options Options) (<-chan string, <-chan error)

	// Send will send a directed message to the specified receivers. If returns
	// two channels, one which returns the id of the sent message and one which
	// returns any errors which occur. If the message requires an ack and a
	// timeout occurs before an ack is received and retries are exceeded,
	// ErrAckTimeout is returned on this channel.
	Send(payload []byte, receivers []string, options Options) (<-chan string, <-chan error)

	// Messages returns a chan to receive directed messages on.
	Messages() <-chan *Message

	// Closed returns a chan which sends a bool when the Vessel has been
	// closed or the client has become disconnected.
	// DEPRECATED: Use Events() instead.
	Closed() <-chan bool // TODO: Remove in a future version

	// Events returns a chan which sends Vessel events triggered on changes to
	// connection state. For example, this is triggered on connect and
	// disconnect. This will buffer a predefined amount of events before
	// dropping them if nothing is reading from the channel.
	Events() <-chan Event

	// Returns a deep copy of the configuration used to set up this Vessel client.
	GetConfig() *Config
}

type vessel struct {
	addrs         []string
	addrIdx       uint32
	config        *Config
	transport     transport
	fallbacks     []transport
	messages      chan *Message
	shutdown      chan bool
	offsetManager *offsetManager
}

// New creates a new Vessel client. It returns an error if the Config specifies
// Transports which are not supported or contains an Option with an invalid
// value. The client connects to one of the provided brokers and cycles
// through the list on reconnects.
func New(addrs []string, config *Config) (Vessel, error) {
	if len(config.Fallbacks) == 0 {
		config.Fallbacks = append(config.Fallbacks, TCP, HTTP)
	}

	config.closed = make(chan bool, 1)
	config.disconnected = make(chan bool, 1)
	config.events = make(chan Event, eventsQLen)

	if config.ClientBufferSize == 0 {
		config.ClientBufferSize = defaultMsgsQLen
	}

	offsetManager := &offsetManager{}

	vessel := &vessel{
		addrs:         addrs,
		addrIdx:       hash(config.ClientID) % uint32(len(addrs)),
		config:        config,
		messages:      make(chan *Message, config.ClientBufferSize),
		shutdown:      make(chan bool),
		fallbacks:     make([]transport, 0, len(config.Fallbacks)),
		offsetManager: offsetManager,
	}

	for _, transport := range config.Fallbacks {
		if factory, ok := transportFactories[transport]; !ok {
			return nil, fmt.Errorf("Transport %s is not supported", transport)
		} else {
			if tr, err := factory(config); err != nil {
				return nil, err
			} else {
				tr.SetOffsetManager(offsetManager)
				vessel.fallbacks = append(vessel.fallbacks, tr)
			}
		}
	}

	return vessel, nil
}

// Dial resolves the vessel at the provided host. It returns an error if
// the vessel is not available, the transport is not supported, or a
// session could not be created. This will attempt to dial each broker until
// a connection is established or every broker has been dialed.
func (v *vessel) Dial() error {
fallbacksLoop:
	for i, transport := range v.fallbacks {
		v.transport = transport
		for attempts := 0; attempts < len(v.addrs); attempts++ {
			if err := v.transport.Dial(v.addrs[v.addrIdx]); err != nil {
				log.Printf("Failed to dial transport %s: %s",
					v.config.Fallbacks[i], err.Error())
				v.addrIdx = (v.addrIdx + 1) % uint32(len(v.addrs))
				if attempts == len(v.addrs)-1 && i == len(v.fallbacks)-1 {
					// Nothing else to fall back to.
					return err
				}
			} else {
				break fallbacksLoop
			}
		}
	}

	// Drain the closed channel in case the client is reconnecting and never
	// received on it.
	v.config.drainDisconnect()

	go v.dispatcher()
	v.config.connect()

	return nil
}

// AttemptDial makes successive calls to Dial until the dial limit is reached
// or a successful dial occurs. A backoff timeout used to wait between each
// dial attempt.
func (v *vessel) AttemptDial(dialLimit int64, backoff time.Duration) error {
	for i := int64(0); i < dialLimit-1; i++ {
		if v.Dial() == nil {
			return nil
		}
		time.Sleep(backoff)
	}
	return v.Dial()
}

// Close terminates the vessel session and performs a graceful shutdown of the
// transport client and kills the dispatcher.
func (v *vessel) Close() error {
	if v.transport == nil {
		return ErrNotConnected
	}

	if err := v.transport.Close(); err != nil {
		return err
	}

	// Kill the dispatcher.
	v.shutdown <- true

	return nil
}

// SessionID returns the ID of the currently connected Session. If the Vessel
// has not been dialed, return a blank string rather than panic.
func (v *vessel) SessionID() string {
	if v.transport == nil {
		return ``
	}
	return v.transport.SessionID()
}

// Subscribe returns a chan which yields messages published on the
// specified vessel Subscriptions. It returns an error if the subscription
// fails.
func (v *vessel) Subscribe(subscription *Subscription) (chan *Message, error) {
	if v.transport == nil {
		return nil, ErrNotConnected
	}
	return v.transport.Subscribe(subscription)
}

// AddSubscriptionsToChannel adds subscriptions to the passed in Go channel.
func (v *vessel) AddSubscriptionsToChannel(messageChannel chan *Message, subscriptions ...*Subscription) error {
	if v.transport == nil {
		return ErrNotConnected
	}
	return v.transport.AddSubscriptionsToChannel(messageChannel, subscriptions...)
}

// Unsubscribe closes the chan returned by Subscribe and unsubscribes the
// client from the channel. It returns an error if the client wasn't subscribed
// to the channel.
func (v *vessel) Unsubscribe(subscription *Subscription) error {
	if v.transport == nil {
		return ErrNotConnected
	}
	return v.transport.Unsubscribe(subscription)
}

// Publish broadcasts the message on the specified channel. It returns two
// channels, one which returns the id of the published message and one which
// returns any errors which occur. If the message requires an ack and a timeout
// occurs before an ack is received and retries are exceeded, ErrAckTimeout is
// returned on this channel.
func (v *vessel) Publish(payload []byte, subscription *Subscription, options Options) (<-chan string, <-chan error) {
	var (
		resultC = make(chan string, 1)
		errorC  = make(chan error, 1)
		ackC    = make(chan *ack, 1)
	)

	if v.transport == nil {
		errorC <- ErrNotConnected
		return resultC, errorC
	}

	channel := subscription.Channel
	id, err := v.transport.Publish(channel, payload, options, ackC)
	if err != nil {
		errorC <- err
		return resultC, errorC
	}

	// Publishes with QoS > 0 require an ack from the server.
	if options.QoS > FireAndForget {
		go v.waitForAck(id, resultC, errorC, ackC, options, func() string {
			newID, _ := v.transport.Publish(channel, payload, options, ackC)
			return newID
		})
	} else {
		resultC <- id
	}

	return resultC, errorC
}

// Send will send a directed message to the specified receivers. If returns two
// channels, one which returns the id of the sent message and one which returns
// any errors which occur. If the message requires an ack and a timeout occurs
// before an ack is received and retries are exceeded, ErrAckTimeout is
// returned on this channel.
func (v *vessel) Send(payload []byte, receivers []string, options Options) (<-chan string, <-chan error) {
	var (
		resultC = make(chan string, 1)
		errorC  = make(chan error, 1)
		ackC    = make(chan *ack, 1)
	)

	if v.transport == nil {
		errorC <- ErrNotConnected
		return resultC, errorC
	}

	id, err := v.transport.Send(payload, receivers, options, ackC)
	if err != nil {
		errorC <- err
		return resultC, errorC
	}

	// Directed messages with QoS > 0 require an ack from each receiver.
	if options.QoS > FireAndForget {
		go v.waitForAck(id, resultC, errorC, ackC, options, func() string {
			newID, _ := v.transport.Send(payload, receivers, options, ackC)
			return newID
		})
	} else {
		resultC <- id
	}

	return resultC, errorC
}

// Messages returns a chan to receive directed messages on.
func (v *vessel) Messages() <-chan *Message {
	return v.messages
}

// Closed returns a chan which sends a bool when the Vessel has been closed or
// the client has become disconnected.
// DEPRECATED: Use Events() instead.
func (v *vessel) Closed() <-chan bool {
	return v.config.closed
}

// Events returns a chan which sends Vessel events triggered on changes to
// connection state. For example, this is triggered on connect and disconnect.
// This will buffer a predefined amount of events before dropping them if
// nothing is reading from the channel.
func (v *vessel) Events() <-chan Event {
	return v.config.events
}

// Returns a deep copy of the configuration used to set up this Vessel client.
func (v *vessel) GetConfig() *Config {
	// Make a deep copy of the current config
	c := Config{}
	c = *v.config
	// Plow over unexported fields. Not accessible by whoever's calling this.
	c.closed = nil
	c.disconnected = nil
	c.events = nil
	return &c
}

// dispatcher runs a loop which provesses Messages received from the Transport.
// It should be called in a goroutine.
func (v *vessel) dispatcher() {
	messages := v.transport.Messages()
	for {
		select {
		case <-v.shutdown:
			return
		case <-v.config.disconnected:
			// Store current offsets to indicate missed messages to read when reconnected
			v.offsetManager.TransferOffsets()

			log.Println("vessel: attempting to reconnect")
			if v.tryReDial() {
				log.Println("vessel: reconnect succeeded")
			} else {
				// Unable to re-connect so exit the dispatcher.
				log.Println("vessel: failed to reconnect")
				v.config.reconnectFailed()
			}
			// If redial succeeded, it will start a new dispatcher.
			return
		case message := <-messages:
			// Directed messages with QoS > 0 require an ack.
			if message.QoS > FireAndForget {
				v.transport.Ack(message.ID, message.Origin)
			}

			if v.config.applyFilters(message) {
				// Don't propagate if filtered.
				continue
			}
			v.messages <- message
		}
	}
}

// Try to redial vessel with a backoff. If successful try to resubscribe the
// previous subscriptions on the existing go channels and return true. If
// failed return false.
func (v *vessel) tryReDial() bool {
	retries := uint(1)
	reconnectWait := v.config.ReconnectDelay

	for {
		if err := v.Dial(); err != nil {
			time.Sleep(reconnectWait)

			if retries > v.config.ReconnectRetries-1 {
				return false
			}

			retries++

			if v.config.ReconnectBackoff {
				reconnectWait *= 2
			}

		} else {
			// Re-subscribe the existing subscriptions.
			if err := v.transport.ReSubscribeAllChannels(); err != nil {
				// TODO: Handle internal errors.
				log.Printf("Failed to resubscribe all channels: %s", err)

				// QUESTION: Die if we can't reconnect all channels?
			}

			return true
		}
	}
}

// waitForAck waits the configured timeout for an ack and handles retrying if
// applicable.
func (v *vessel) waitForAck(id string, resultC chan<- string, errorC chan<- error,
	ackC <-chan *ack, options Options, retry func() string) {

	timeout := options.AckTimeout
	if timeout <= 0 {
		timeout = defaultAckTimeout
	}

	// Wait for ack.
	select {
	case <-time.After(timeout):
		break
	case <-ackC:
		resultC <- id
		return
	}

	v.transport.RemoveAck(id)

	// Attempt retries.
	for i := uint(0); i < options.Retries; i++ {
		id = retry()
		select {
		case <-time.After(timeout):
			break
		case <-ackC:
			resultC <- id
			return
		}

		v.transport.RemoveAck(id)
		if options.RetryBackoff {
			timeout *= 2
		}
	}

	// Exceeded retries, no ack.
	errorC <- ErrAckTimeout
}

func hash(clientID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(clientID))
	return h.Sum32()
}
