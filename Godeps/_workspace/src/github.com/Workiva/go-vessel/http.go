package vessel

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/batcher"
)

const (
	// HTTP is the name of the HTTP transport.
	HTTP Transport = "http"

	// OptionPollInterval specifies the HTTP polling interval for a Transport.
	// The default is 2 seconds.
	OptionPollInterval Option = "poll-interval"

	// OptionFailStop specifies the number of failed poll attempts before
	// disconnecting the client. The default is 5.
	OptionPollFailStop Option = "poll-fail-stop"

	// OptionPollSize specifies the maximum number of messages that should
	// be returned in a single poll. The default is 100.
	OptionPollSize Option = "poll-size"

	defaultPollInterval = 2 * time.Second
	defaultPollFailStop = 5
	defaultPollSize     = 100

	batchingHeader = "X-Vessel-Batch"
)

// httpTransport implements the Transport interface using HTTP as the
// underlying transport layer.
type httpTransport struct {
	*baseTransport
	addr            string
	subscriptionsMu sync.Mutex
	subscriptions   map[string]*subscription
	acksMu          sync.Mutex
	acks            map[string]chan<- *Envelope
	listen          bool
	pollInterval    time.Duration
	pollFailStop    uint
	pollSize        uint32
	failedPolls     uint
}

// NewHTTPTransport creates and returns a new Transport which communicates
// with a vessel using HTTP.
func newHTTPTransport(config *Config) (transport, error) {
	transport := &httpTransport{
		baseTransport: newBaseTransport(config),
		subscriptions: map[string]*subscription{},
		acks:          map[string]chan<- *Envelope{},
		listen:        true,
		pollInterval:  defaultPollInterval,
		pollFailStop:  defaultPollFailStop,
	}

	if err := transport.setOptions(config.Options); err != nil {
		return nil, err
	}

	return transport, nil
}

// Dial resolves the vessel at the provided host. It returns an error if the
// vessel is not available, the transport is not supported, or a session could
// not be created.
func (h *httpTransport) Dial(host string) error {
	if err := h.dial(host); err != nil {
		return err
	}

	addr, err := h.resolveHTTP(host)
	if err != nil {
		return err
	}

	if h.config.Proxy {
		h.addr = host
	} else {
		h.addr = addr
	}

	if h.listen {
		go h.dispatchLoop()
		// If offsets are nil, server doesn't support offset tracking
		// resort to a lot of polls
		if h.offsetManager.SupportsOffsetTracking() {
			go h.pollEverythingLoop()
		} else {
			go h.clientLoop()
		}
		if h.config.batching() {
			go h.sendLoop()
		}
	}

	return nil
}

// Close terminates the vessel session and performs a graceful shutdown of
// the transport client.
func (h *httpTransport) Close() error {
	h.baseTransport.Close()
	return nil
}

// Subscribe returns a chan which yields messages published on the
// specified vessel Subscriptions. It returns an error if the subscription
// fails.
func (h *httpTransport) Subscribe(subscription *Subscription) (chan *Message, error) {
	if subscription.BufferSize == 0 {
		subscription.BufferSize = defaultSubsQLen
	}
	msgs := make(chan *Message, subscription.BufferSize)
	err := h.AddSubscriptionsToChannel(msgs, subscription)
	return msgs, err
}

// AddSubscriptionsToChannel adds subscriptions to the passed in Go channel.
func (h *httpTransport) AddSubscriptionsToChannel(messageChannel chan *Message,
	subscriptions ...*Subscription) error {

	startPolling := []*Subscription{}
	for _, sub := range subscriptions {
		if !h.isSubscribed(sub.Channel) {
			startPolling = append(startPolling, sub)
		}
	}

	ackChan := make(chan *ack, 1)
	env := h.newEnvelope(subscriptionMsg, []byte{subscribe}, nil,
		h.getChannelString(subscriptions), Options{QoS: AtLeastOnce})
	if err := h.handleSendEnvelope(env, ackChan); err != nil {
		return err
	}

	if err := h.waitForSubscriptionAck(ackChan, subscriptions); err != nil {
		return err
	}

	h.addSubscriptionsToChannel(messageChannel, subscriptions)

	for _, sub := range startPolling {
		// Don't spawn subscription loop if tracking offsets,
		// as that uses the "poll everything" function
		if !h.offsetManager.SupportsOffsetTracking() {
			go h.subscriptionLoop(sub)
		}
	}

	return nil
}

// Resubscribe all existing channel subscriptions.
func (h *httpTransport) ReSubscribeAllChannels() error {
	messageChans := h.getAndClearSubscriptionsByChannel()

	for messageChan, subs := range messageChans {
		if len(subs) > 0 {
			if err := h.AddSubscriptionsToChannel(messageChan, subs...); err != nil {
				return err
			}
		}
	}

	return nil
}

// Unsubscribe closes the chan returned by Subscribe and unsubscribes the
// client from the channel. It returns an error if the client wasn't subscribed
// to the channel.
func (h *httpTransport) Unsubscribe(subscription *Subscription) error {
	if err := h.unsubscribe(subscription); err != nil {
		return err
	}
	env := h.newEnvelope(subscriptionMsg, []byte{unsubscribe}, nil,
		subscription.Channel, Options{QoS: FireAndForget})
	return h.handleSendEnvelope(env, nil)
}

// Publish broadcasts the message on the specified channel. It returns the id
// of the published envelope and an error if the publish failed.
func (h *httpTransport) Publish(channel string, payload []byte, options Options, ackChan chan<- *ack) (string, error) {
	env := h.newEnvelope(publishMsg, payload, nil, channel, options)
	return env.ID, h.handleSendEnvelope(env, ackChan)
}

// Send will send a directed message to the specified receivers. It returns
// the id of the sent message and an error if the send failed.
func (h *httpTransport) Send(payload []byte, receivers []string, options Options, ackChan chan<- *ack) (string, error) {
	env := h.newEnvelope(directedMsg, payload, receivers, "", options)
	return env.ID, h.handleSendEnvelope(env, ackChan)
}

// Ack sends an ack message in response to a received message.
func (h *httpTransport) Ack(id, receiver string) error {
	return h.handleSendEnvelope(h.newAck(id, receiver), nil)
}

func (h *httpTransport) sendLoop() {
	h.doneSending = make(chan bool, 1)
	defer func() {
		h.doneSending <- true
	}()

	for {
		h.closedMu.RLock()
		closed := h.closed
		h.closedMu.RUnlock()
		if closed {
			// Expected, so return
			return
		}

		batch, err := h.batcher.Get()
		if err != nil {
			if err == batcher.ErrDisposed {
				return
			}
			log.Println("httpTransport.sendLoop batcher:", err.Error())
			return
		}
		if len(batch) == 0 {
			continue
		}

		envelopes := make([]*Envelope, len(batch))
		for i, v := range batch {
			envelopes[i] = v.(*Envelope)
		}
		if err := h.sendEnvelopes(envelopes...); err != nil {
			log.Println(err)
		}
	}
}

func (h *httpTransport) handleSendEnvelope(envelope *Envelope, ackChan chan<- *ack) error {
	if envelope.QoS > FireAndForget {
		h.addAck(envelope, ackChan)
	}

	// If not batching, or shouldn't batch envelope
	if !h.config.batching() || envelope.Type == ackMsg || envelope.Type == subscriptionMsg {
		return h.sendEnvelopes(envelope)
	}
	return h.batcher.Put(envelope)
}

func (h *httpTransport) sendEnvelopes(envelopes ...*Envelope) error {
	var data []byte
	var err error
	switch {
	case len(envelopes) == 0:
		// Nothing to send
		return nil
	case !h.config.batching():
		// If not batching, there should be only one thing
		data, err = h.marshaler.Marshal(envelopes[0])
	default:
		// We must be batching
		data, err = h.marshaler.MarshalArray(envelopes)
	}

	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", h.addr, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	for name, value := range h.getHttpTransportHeaders() {
		req.Header.Set(name, value)
	}
	if h.config.batching() {
		req.Header.Set(batchingHeader, "true")
	} else {
		req.Header.Set(batchingHeader, "false")
	}

	resp, err := h.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Non-200 response code: %d", resp.StatusCode)
	}

	return nil
}

func (h *httpTransport) clientLoop() {
	for {
		if h.isClosed() {
			return
		}

		if err := h.pollClientMessages(); err != nil {
			log.Println("Client poll failed:", err.Error())
			h.failedPolls++
			if h.failedPolls >= h.pollFailStop {
				log.Println("Exceeded poll attempts, disconnecting")
				h.config.disconnected <- true
				h.baseTransport.Close()
				return
			}
		} else {
			h.failedPolls = 0
		}

		time.Sleep(h.pollInterval)
	}
}

func (h *httpTransport) subscriptionLoop(sub *Subscription) {
	for {
		if h.isClosed() {
			// Transport closed. No need to notify on error channel.
			return
		}

		internalSub, err := h.getSubscription(sub.id)
		if err != nil {
			// Subscription missing from map. This means it was removed by
			// Unsubscribe. No need to signal on error channel.
			return
		}

		if err := h.pollSubscription(internalSub); err != nil {
			log.Println("Subscription poll failed:", err.Error())
			internalSub.failCount++
			if internalSub.failCount >= h.pollFailStop {
				pollError := fmt.Errorf("Exceeded poll attempts, disconnecting")
				log.Println(pollError)
				h.config.disconnected <- true
				h.baseTransport.Close()
				return
			}
		} else {
			internalSub.failCount = 0
		}

		time.Sleep(h.pollInterval)
	}
}

// pollEverythingLoop continually calls pollEverything with appropriate handling
// around the frequency of polls and failed polls
func (h *httpTransport) pollEverythingLoop() {
	for {
		if h.isClosed() {
			return
		}

		if err := h.pollEverything(); err != nil {
			log.Println("poll failed:", err.Error())
			h.failedPolls++
			if h.failedPolls >= h.pollFailStop {
				log.Println("exceeded poll attempts, disconnecting")
				h.config.disconnected <- true
				h.baseTransport.Close()
				return
			}
		} else {
			h.failedPolls = 0
		}

		time.Sleep(h.pollInterval)
	}
}

// pollEverything polls the "/poll" endpoint on a vessel server.
// This polls for directed messages and publish/subscribe messages based
// on provided offsets of the messages last received by this client
func (h *httpTransport) pollEverything() error {
	offsetsStr := h.offsetManager.GetOffsetStringMap()
	request := &PollRequest{Count: h.pollSize, PartitionOffsets: offsetsStr}
	payload, err := h.marshaler.MarshalPollRequest(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/v1/poll", h.addr), bytes.NewBuffer(payload))

	for name, value := range h.getHttpTransportHeaders() {
		req.Header.Set(name, value)
	}

	resp, err := h.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("poll returned status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	response, err := h.marshaler.UnmarshalPollResponse(data)
	if err != nil {
		return err
	}

	h.offsetManager.UpdateOffsetsFromStringMap(response.PartitionOffsets)
	for _, envelope := range response.Envelopes {
		h.envelopes <- envelope
	}

	return nil
}

func (h *httpTransport) pollClientMessages() error {
	// Deprecated, this should be removed in vessel 3.0
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/client/%s", h.addr, h.config.ClientID), nil)
	if err != nil {
		return err
	}
	for name, value := range h.getHttpTransportHeaders() {
		req.Header.Set(name, value)
	}

	resp, err := h.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Client poll returned status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	envelopes, err := h.marshaler.UnmarshalArray(body)
	if err != nil {
		return err
	}

	// Don't need to update offsets here, as this isn't used if the server
	// supports offset tracking
	for _, envelope := range envelopes {
		h.envelopes <- envelope
	}

	return nil
}

func (h *httpTransport) pollSubscription(subscription *subscription) error {
	// Deprecated, this should be removed in vessel 3.0
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/subscription/%s", h.addr,
		subscription.id), nil)
	if err != nil {
		return err
	}
	for name, value := range h.getHttpTransportHeaders() {
		req.Header.Set(name, value)
	}

	resp, err := h.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Subscription poll returned status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	envelopes, err := h.marshaler.UnmarshalArray(body)
	if err != nil {
		return err
	}

	// Don't need to update offsets here, as this isn't used if the server
	// supports offset tracking
	for _, envelope := range envelopes {
		h.envelopes <- envelope
	}

	return nil
}

func (h *httpTransport) resolveHTTP(addr string) (string, error) {
	meta, err := h.resolveTransport(addr, HTTP)
	if err != nil {
		return "", err
	}

	port, ok := meta["port"]
	if !ok {
		return "", fmt.Errorf("Could not resolve transport: %s", addr)
	}

	url, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	host := strings.Split(url.Host, ":")[0]

	return fmt.Sprintf("%s://%s:%s%s", url.Scheme, host, port, url.Path), nil
}

func (h *httpTransport) setOptions(options map[Option]interface{}) error {
	for option, value := range options {
		switch option {
		case OptionPollInterval:
			if interval, ok := value.(time.Duration); !ok {
				return errors.New("OptionPollInterval value must be a time.Duration")
			} else {
				h.pollInterval = interval
			}
		case OptionPollFailStop:
			if failStop, ok := value.(uint); !ok {
				return errors.New("OptionPollFailStop value must be a uint")
			} else {
				h.pollFailStop = failStop
			}
		case OptionPollSize:
			if size, ok := value.(uint32); !ok {
				return errors.New("OptionPollSize must be a uint32")
			} else {
				h.pollSize = size
			}
		default:
			log.Printf("HTTP transport ignoring option %s\n", option)
		}
	}
	return nil
}

func (h *httpTransport) getHttpTransportHeaders() map[string]string {
	headers := h.getHeaders()
	headers[protocolHeader] = string(HTTP)
	headers["Content-Type"] = string(h.config.ContentType)
	return headers
}
