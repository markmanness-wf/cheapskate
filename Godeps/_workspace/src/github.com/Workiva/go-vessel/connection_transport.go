package vessel

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"github.com/Workiva/go-datastructures/batcher"
)

type proto uint16

const (
	handshake proto = iota
	envel

	// OptionTLSConfig specifies the tls.Config to use for TLS transport
	// connections.
	OptionTLSConfig Option = "tls-config"
)

// Connection interface for communicating with a vessel server
type conn interface {
	// Close the connection
	Close() error
	// Send data via the connection
	Send([]byte, proto) error
	// Receive data from the connection
	Recv() ([]byte, error)
}

// connectionTransport implements most of the functionality for transports that
// can send and receive data. Structs that extend this must implement a Dial()
// function that sets conn to the appropriate connection.
type connectionTransport struct {
	*baseTransport
	conn          conn
	listen        bool
	tls           *tls.Config
	stopHeartbeat chan bool
}

// newConnectionTransport creates a new connectionTransport.
func newConnectionTransport(config *Config) (*connectionTransport, error) {
	transport := &connectionTransport{
		baseTransport: newBaseTransport(config),
		listen:        true,
		stopHeartbeat: make(chan bool),
	}

	if err := transport.setOptions(config.Options); err != nil {
		return nil, err
	}

	return transport, nil
}

// heartbeat is used to continuously ping the vessel server at a regular interval.
// The interval is defined by the HeartbeatInterval config property or defaults to
// the defaultHeartbeatInterval const if not defined.
func (c *connectionTransport) heartbeat() {
	if c.config.HeartbeatInterval <= 0 {
		c.config.HeartbeatInterval = defaultHeartbeatInterval
	}

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	for {
		select {
		case <-ticker.C:
			// send ping to server
			if err := c.sendEnvelopes(c.newEnvelope(heartbeatMsg, []byte("ping"), nil, "", Options{})); err != nil {
				log.Println("vessel:", err)
			}
		case <-c.stopHeartbeat:
			ticker.Stop()
			log.Println("vessel:Heartbeat stopped...")
			return
		}
	}
}

// handshake sends a handshake message to the vessel server
func (c *connectionTransport) handshake() error {
	handshakeJson, err := json.Marshal(c.getHandshake())
	if err != nil {
		return err
	}

	// Don't batch handshakes
	return c.conn.Send(handshakeJson, handshake)
}

// Close terminates the vessel session and performs a graceful shutdown of
// the transport client.
func (c *connectionTransport) Close() error {
	c.baseTransport.Close()
	select {
	case c.stopHeartbeat <- true:
	default:
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Subscribe returns a chan which yields messages published on the
// specified vessel Subscriptions. It returns an error if the subscription
// fails.
func (c *connectionTransport) Subscribe(subscription *Subscription) (chan *Message, error) {
	if subscription.BufferSize == 0 {
		subscription.BufferSize = defaultSubsQLen
	}
	messages := make(chan *Message, subscription.BufferSize)
	err := c.AddSubscriptionsToChannel(messages, subscription)
	return messages, err
}

// AddSubscriptionsToChannel adds subscriptions to the passed in Go channel.
// It returns an error if the client wasn't subscribed to the channel or the
// unsubscribe failed.
func (c *connectionTransport) AddSubscriptionsToChannel(messageChannel chan *Message, subscriptions ...*Subscription) error {
	ackChan := make(chan *ack, 1)
	envelope := c.newEnvelope(subscriptionMsg, []byte{subscribe}, nil,
		c.getChannelString(subscriptions), Options{QoS: AtLeastOnce})
	if err := c.handleSendEnvelope(envelope, ackChan); err != nil {
		return err
	}

	if err := c.waitForSubscriptionAck(ackChan, subscriptions); err != nil {
		return err
	}

	c.addSubscriptionsToChannel(messageChannel, subscriptions)
	return nil
}

// Unsubscribe closes the chan returned by Subscribe and unsubscribes the
// client from the channel. It returns an error if the client wasn't
// subscribed to the channel.
func (c *connectionTransport) Unsubscribe(subscription *Subscription) error {
	if err := c.unsubscribe(subscription); err != nil {
		return err
	}
	envelope := c.newEnvelope(subscriptionMsg, []byte{unsubscribe}, nil,
		subscription.Channel, Options{QoS: FireAndForget})
	return c.handleSendEnvelope(envelope, nil)
}

// Resubscribe all existing channel subscriptions.
func (c *connectionTransport) ReSubscribeAllChannels() error {
	messageChans := c.getAndClearSubscriptionsByChannel()

	for messageChan, subscriptions := range messageChans {
		if len(subscriptions) > 0 {
			if err := c.AddSubscriptionsToChannel(messageChan, subscriptions...); err != nil {
				return err
			}
		}
	}

	return nil
}

// Publish broadcasts the message on the specified channel. It returns the
// id of the published message and an error if the publish failed.
func (c *connectionTransport) Publish(channel string, payload []byte, options Options, ackChan chan<- *ack) (string, error) {
	envelope := c.newEnvelope(publishMsg, payload, nil, channel, options)
	return envelope.ID, c.handleSendEnvelope(envelope, ackChan)
}

// Send will send a directed message to the specified receivers. It returns
// the id of the sent message and an error if the send failed.
func (c *connectionTransport) Send(payload []byte, receivers []string, options Options, ackChan chan<- *ack) (string, error) {
	envelope := c.newEnvelope(directedMsg, payload, receivers, "", options)
	return envelope.ID, c.handleSendEnvelope(envelope, ackChan)
}

// Ack sends an ack message in response to a received message.
func (c *connectionTransport) Ack(id string, receiver string) error {
	return c.handleSendEnvelope(c.newAck(id, receiver), nil)
}

// setOptions sets options on the transport from the given options, ignoring
// those that don't apply
func (c *connectionTransport) setOptions(options map[Option]interface{}) error {
	for option, value := range options {
		switch option {
		case OptionTLSConfig:
			config, ok := value.(*tls.Config)
			if !ok {
				return errors.New("OptionTLSConfig value must be a *tls.Config")
			}
			c.tls = config
		default:
			log.Println("Transport ignoring option", option)
		}
	}

	return nil
}

func (c *connectionTransport) handleSendEnvelope(envelope *Envelope, ackChan chan<- *ack) error {
	if envelope.QoS > FireAndForget {
		c.addAck(envelope, ackChan)
	}

	// If not batching, or shouldn't batch envelope
	if !c.config.batching() || envelope.Type == ackMsg || envelope.Type == subscriptionMsg {
		return c.sendEnvelopes(envelope)
	}
	return c.batcher.Put(envelope)
}

// sendEnvelopes sends envelopes to the server
func (c *connectionTransport) sendEnvelopes(envelopes ...*Envelope) error {
	var data []byte
	var err error
	switch {
	case len(envelopes) == 0:
		// Nothing to send
		return nil
	case !c.config.batching():
		// If not batching, there can be only one
		if len(envelopes) > 1 {
			panic("connectionTransport.sendEnvelopes: more than one items being sent while not batching")
		}
		data, err = c.marshaler.Marshal(envelopes[0])
	default:
		// We must be batching
		data, err = c.marshaler.MarshalArray(envelopes)
	}

	if err != nil {
		return err
	}

	return c.conn.Send(data, envel)
}

func (c *connectionTransport) sendLoop() {
	c.doneSending = make(chan bool, 1)
	defer func() {
		c.doneSending <- true
	}()

	for {
		c.closedMu.RLock()
		closed := c.closed
		c.closedMu.RUnlock()
		if closed {
			// Expected, so return
			return
		}

		batch, err := c.batcher.Get()
		if err != nil {
			if err == batcher.ErrDisposed {
				return
			}
			log.Println("connectionTransport.sendLoop batcher:", err.Error())
			return
		}
		if len(batch) == 0 {
			continue
		}

		envelopes := make([]*Envelope, len(batch))
		for i, v := range batch {
			envelopes[i] = v.(*Envelope)
		}
		if err := c.sendEnvelopes(envelopes...); err != nil {
			log.Println("vessel:", err)
		}
	}
}

// recvLoop waits to receive messages, checking if the server becomes
// disconnected or the connection is closed
func (c *connectionTransport) recvLoop() {
	for {
		if err := c.recv(); err != nil {
			if err == io.EOF {
				// Server disconnected
				log.Println("Server disconnected:", err.Error())
				c.config.disconnected <- true
				c.Close()
				return
			}

			c.closedMu.RLock()
			closed := c.closed
			c.closedMu.RUnlock()
			if closed {
				// Expected, so return
				return
			}
			log.Println("vessel:", err)
		}
	}
}

// recv receives a message from the server
func (c *connectionTransport) recv() error {
	data, err := c.conn.Recv()
	if err != nil {
		return err
	}

	// Return on handshake confirmations (websockets)
	if strings.HasPrefix(string(data), "handshake:") {
		return nil
	}

	// TODO get pooling back somehow?
	if !c.config.batching() {
		envelope, err := c.marshaler.Unmarshal(data)
		if err != nil {
			return err
		}
		c.offsetManager.UpdateOffsets([]*Envelope{envelope})
		c.envelopes <- envelope
		return nil
	}

	envelopes, err := c.marshaler.UnmarshalArray(data)
	if err != nil {
		return err
	}
	c.offsetManager.UpdateOffsets(envelopes)
	for _, envelope := range envelopes {
		c.envelopes <- envelope
	}

	return nil
}
