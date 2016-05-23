package vessel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Workiva/BoomFilters"
	"github.com/Workiva/go-datastructures/batcher"
	"github.com/mattrobenolt/gocql/uuid"
)

const (
	timeBeforeExpiration     = 15 * time.Second
	clientHeader             = "X-Vessel-Client"
	sessionHeader            = "X-Vessel-Session"
	subscriptionHeader       = "X-Vessel-Subscription"
	protocolHeader           = "X-Vessel-Protocol"
	clientVersionHeader      = "X-Vessel-Client-Version"
	defaultLowerQLen         = 200000
	defaultSubsQLen          = 100000
	defaultMsgsQLen          = 100000
	defaultHeartbeatInterval = 15 * time.Second
	filterCapacity           = 10000
	defaultTimeout           = 5 * time.Second
)

var transportFactories = map[Transport]func(*Config) (transport, error){
	TCP:  newTCPTransport,
	HTTP: newHTTPTransport,
	WS:   newWSTransport,
}

type subscription struct {
	id        string
	channel   string
	messages  chan *Message
	refCount  *uint32
	refMu     *sync.Mutex
	failCount uint
}

func (s *subscription) unsubscribe() {
	s.refMu.Lock()
	(*s.refCount)--
	if *s.refCount <= 0 {
		close(s.messages)
	}
	s.refMu.Unlock()
}

type ack struct {
	success bool
	headers http.Header
}

type ackContext struct {
	acks uint
	need uint
	done chan<- *ack
}

// transport communicates with a vessel using a particular protocol.
type transport interface {
	// Dial resolves the vessel at the provided host. It returns an error if
	// the vessel is not available, the transport is not supported, or a
	// session could not be created.
	Dial(addr string) error

	// Close terminates the vessel session and performs a graceful shutdown of
	// the transport client.
	Close() error

	// SessionID returns a unique identifier of the currently connected
	// session.
	SessionID() string

	// Subscribe returns a chan which yields messages published on the
	// specified vessel Subscriptions. It returns an error if the subscription
	// fails.
	Subscribe(subscription *Subscription) (chan *Message, error)

	// AddSubscriptionsToChannel adds subscriptions to the passed in Go channel.
	// It returns an error if the client wasn't subscribed to the channel or the
	// unsubscribe failed.
	AddSubscriptionsToChannel(messageChannel chan *Message, subscriptions ...*Subscription) error

	// Unsubscribe closes the chan returned by Subscribe and unsubscribes the
	// client from the channel. It returns an error if the client wasn't
	// subscribed to the channel.
	Unsubscribe(subscription *Subscription) error

	// Publish broadcasts the message on the specified channel. It returns the
	// id of the published message and an error if the publish failed.
	Publish(channel string, payload []byte, options Options, ackChan chan<- *ack) (string, error)

	// Send will send a directed message to the specified receivers. It returns
	// the id of the sent message and an error if the send failed.
	Send(payload []byte, receivers []string, options Options, ackChan chan<- *ack) (string, error)

	// Ack sends an ack message in response to a received message.
	Ack(id, receiver string) error

	// Messages returns a chan which yields messages directed to this client.
	Messages() <-chan *Message

	// RemoveAck removes the ack context for the specified id.
	RemoveAck(id string)

	// Resubscribe all existing channel subscriptions.
	ReSubscribeAllChannels() error

	SetOffsetManager(*offsetManager)
}

// baseTransport implements base Transport functionality, such as session
// management and envelope pooling. Transport implementations should embed this
// struct.
type baseTransport struct {
	sessionID       string
	host            string
	mu              sync.RWMutex
	shutdown        chan bool
	pool            *sync.Pool
	closedMu        sync.RWMutex
	closed          bool
	envelopes       chan *Envelope
	subscriptionsMu sync.RWMutex
	subscriptions   map[string]*subscription
	acksMu          sync.Mutex
	acks            map[string]*ackContext
	messages        chan *Message
	config          *Config
	filter          boom.Filter
	doneSending     chan bool
	batcher         batcher.Batcher
	marshaler       marshaler
	offsetManager   *offsetManager
	killDispatcher  chan bool
}

// newBaseTransport creates a new baseTransport. To start it, dial must be
// called. Otherwise, it's considered in an "invalid" state.
func newBaseTransport(config *Config) *baseTransport {
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if config.Header == nil {
		config.Header = http.Header{}
	}

	if config.InternalBufferSize == 0 {
		config.InternalBufferSize = defaultLowerQLen
	}

	return &baseTransport{
		config:         config,
		shutdown:       make(chan bool, 1),
		pool:           &sync.Pool{New: func() interface{} { return &Envelope{} }},
		envelopes:      make(chan *Envelope, defaultLowerQLen),
		subscriptions:  map[string]*subscription{},
		acks:           map[string]*ackContext{},
		messages:       make(chan *Message, config.InternalBufferSize),
		filter:         boom.NewInverseBloomFilter(filterCapacity),
		killDispatcher: make(chan bool, 1),
	}
}

func (b *baseTransport) SetOffsetManager(offsetManager *offsetManager) {
	b.offsetManager = offsetManager
}

// SessionID exposes the sessionID of this transport.
func (b *baseTransport) SessionID() string {
	b.mu.RLock()
	id := b.sessionID
	b.mu.RUnlock()
	return id
}

// Messages returns a chan which yields messages directed to this client.
func (b *baseTransport) Messages() <-chan *Message {
	return b.messages
}

// newEnvelope populates a new envelope. This envelope is tied to the
// baseTransport and should be returned to the Pool once it's doing being used.
func (b *baseTransport) newEnvelope(envType MessageType, body []byte, receivers []string,
	channel string, options Options) *Envelope {

	env := b.pool.Get().(*Envelope)
	uuid := id().String()
	env.ID = strings.Replace(uuid, "-", "", -1)
	env.Type = envType
	env.Body = body
	env.Receivers = receivers
	env.Channel = channel
	env.QoS = options.QoS
	env.TTL = int64(options.TTL.Seconds())
	env.Vector = []string{b.config.ClientID}
	env.Timestamp = timestamp().Unix()
	if options.Header != nil {
		var b bytes.Buffer
		options.Header.Write(&b)
		env.Headers = b.String()
	}
	return env
}

// newAck populates a new ack envelope.
func (b *baseTransport) newAck(id, receiver string) *Envelope {
	body := make([]byte, len(id)+1)
	body[0] = ackSuccess
	for i, b := range []byte(id) {
		body[1+i] = b
	}
	return b.newEnvelope(ackMsg, body, []string{receiver}, "", Options{QoS: FireAndForget})
}

// dial prepares the baseTransport for use.
func (b *baseTransport) dial(host string) error {
	b.host = host
	err := b.createSession()
	if err == nil {
		b.closed = false
	}
	return err
}

// hasSession indicates if the baseTransport has setup a session.
func (b *baseTransport) hasSession() bool {
	b.mu.RLock()
	hasSession := b.config.ClientID != "" && b.sessionID != ""
	b.mu.RUnlock()
	return hasSession
}

// getHandshake returns the payload for persistent transport handshakes.
func (b *baseTransport) getHandshake() map[string]interface{} {
	b.mu.RLock()
	readbackOffsets := b.offsetManager.GetReadbackOffsetStringMap()
	handshake := map[string]interface{}{
		"client_id":         b.config.ClientID,
		"session_id":        b.sessionID,
		"batch_size":        b.config.BatchMaxMessages,
		"batch_bytes":       b.config.BatchMaxBytes,
		"batch_timeout":     b.config.BatchMaxTime.Nanoseconds() / 1000000,
		"content_type":      b.config.ContentType,
		"partition_offsets": readbackOffsets,
		// TODO remove in Vessel 3
		"supports_offset_tracking": true,
	}
	b.mu.RUnlock()
	return handshake
}

// getHeaders returns a map containing header key-value pairs to use on HTTP
// requests.
func (b *baseTransport) getHeaders() map[string]string {
	b.mu.RLock()
	headers := map[string]string{
		clientHeader:  b.config.ClientID,
		sessionHeader: b.sessionID,
	}
	b.mu.RUnlock()
	return headers
}

// createSession creates a new session with the vessel. This must be done
// before messaging can occur.
func (b *baseTransport) createSession() error {
	header := b.config.Header
	header.Add(clientHeader, b.config.ClientID)
	if expiration, err := b.createOrRenewSession(header); err != nil {
		return err
	} else {
		go b.sessionTicker(expiration)
	}
	return nil
}

// renewSession renews an active vessel session to prevent it from expiring.
func (b *baseTransport) renewSession() (time.Time, error) {
	header := b.config.Header
	header.Add(clientHeader, b.config.ClientID)
	b.mu.RLock()
	header.Add(sessionHeader, b.sessionID)
	b.mu.RUnlock()
	if expiration, err := b.createOrRenewSession(header); err != nil {
		return time.Time{}, err
	} else {
		data := []byte(strconv.FormatInt(expiration.Unix(), 10))
		b.config.emitEvent(Event{Type: EventSessionRenewed, Data: data})
		return expiration, nil
	}
}

// createOrRenewSession creates or renews a vessel session depending on the
// Header contents.
func (b *baseTransport) createOrRenewSession(header http.Header) (time.Time, error) {
	req, err := http.NewRequest("PUT", b.host+"/session", nil)
	if err != nil {
		return time.Time{}, err
	}
	header.Set(clientVersionHeader, getClientVersion())
	req.Header = header
	resp, err := b.config.HTTPClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return time.Time{}, errors.New("Failed to create or renew Vessel session, authentication failed")
	default:
		return time.Time{}, fmt.Errorf("Failed to create or renew Vessel session, error %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return time.Time{}, err
	}

	sessionID, ok := data["id"]
	if !ok {
		return time.Time{}, errors.New("Failed to create or renew Vessel session, no session id returned")
	}

	expiration := time.Unix(int64(data["expiration"].(float64)), 0)

	b.mu.Lock()
	b.sessionID = sessionID.(string)
	b.mu.Unlock()
	return expiration, nil
}

// sessionTicker waits the configured interval before attempting to renew the
// session.
func (b *baseTransport) sessionTicker(expiration time.Time) {
	renew := time.After(expiration.Add(-timeBeforeExpiration).Sub(time.Now()))
	for {
		select {
		case <-renew:
			if newExpiration, err := b.renewSession(); err != nil {
				log.Printf("vessel: %s", err)
				return
			} else {
				renew = time.After(newExpiration.Add(-timeBeforeExpiration).Sub(time.Now()))
			}
		case <-b.shutdown:
			return
		}
	}
}

// Return the subscription channels for the transport keyed by their message go
// channel.
func (b *baseTransport) getAndClearSubscriptionsByChannel() map[chan *Message][]*Subscription {
	channels := make(map[chan *Message][]subscription)

	b.subscriptionsMu.Lock()

	for _, sub := range b.subscriptions {
		chanList, ok := channels[sub.messages]
		if !ok {
			chanList = []subscription{subscription{id: sub.id, channel: sub.channel}}
		} else {
			chanList = append(chanList, subscription{id: sub.id, channel: sub.channel})
		}
		channels[sub.messages] = chanList
	}

	// Clear the subscriptions out.
	b.subscriptions = make(map[string]*subscription)

	b.subscriptionsMu.Unlock()

	channelSubs := make(map[chan *Message][]*Subscription)

	for messageChan, subs := range channels {
		subscriptions := make([]*Subscription, len(subs))

		for i, sub := range subs {
			subscriptions[i] = &Subscription{Channel: sub.channel, id: sub.id}
		}

		channelSubs[messageChan] = subscriptions
	}

	return channelSubs
}

// transport models the transport payload provided by the vessel /info
// endpoint.
type transportInfo struct {
	Protocol Transport              `json:"protocol"`
	Meta     map[string]interface{} `json:"meta"`
}

// info models the payload provided by the vessel /info endpoint.
type info struct {
	Version    version          `json:"version"`
	Transports []transportInfo  `json:"transports"`
	Offsets    map[string]int64 `json:"partition_offsets"`
}

func (i info) GetOffsets() map[int32]int64 {
	if len(i.Offsets) == 0 {
		return nil
	}

	offsets := make(map[int32]int64, len(i.Offsets))
	for key, value := range i.Offsets {
		numKey, err := strconv.ParseInt(key, 10, 32)
		if err != nil {
			log.Println("offset map:", err.Error())
			continue
		}
		offsets[int32(numKey)] = value
	}
	return offsets
}

type version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

func (v version) atLeast(ov version) bool {
	if v.Major != ov.Major {
		return v.Major > ov.Major
	}
	return v.Minor >= ov.Minor
}

// resolveTransport resolves the specified transport protocol at the vessel
// with the given address and returns the metadata for it. An error is returned
// if the transport can't be resolved or the client version is incompatible
// with the vessel version.
func (b *baseTransport) resolveTransport(addr string, transport Transport) (map[string]interface{}, error) {
	if !b.hasSession() {
		return nil, errors.New("Must create a session before resolving transport")
	}

	req, err := http.NewRequest("GET", addr+"/info", nil)
	if err != nil {
		return nil, err
	}
	for name, value := range b.getHeaders() {
		req.Header.Add(name, value)
	}

	resp, err := b.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Could not resolve transport: %s", addr)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info info

	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	// Check if server supports batching
	if !info.Version.atLeast(batchVersion) {
		log.Println("vessel: server doesn't support batching")
		b.config.BatchMaxMessages = 0
		b.config.BatchMaxBytes = 0
		b.config.BatchMaxTime = 0
		b.config.ContentType = JSONContent
	}

	b.offsetManager.SetOffsets(info.GetOffsets())

	if b.config.batching() {
		b.batcher, err = batcher.New(b.config.BatchMaxTime, b.config.BatchMaxMessages,
			b.config.BatchMaxBytes, b.config.BatchQueueLen, b.config.BatchCalcBytes)
		if err != nil {
			return nil, err
		}
	}

	if b.marshaler, err = newMarshaler(b.config); err != nil {
		return nil, err
	}

	for _, tr := range info.Transports {
		if tr.Protocol == transport {
			return tr.Meta, nil
		}
	}

	return nil, fmt.Errorf("Could not resolve transport: %s", addr)
}

func (b *baseTransport) Close() error {
	// Make sure everything from the batcher has been sent
	if b.batcher != nil {
		b.batcher.Dispose()
		if b.doneSending != nil {
			<-b.doneSending
		}
	}

	b.closedMu.Lock()
	b.closed = true
	b.closedMu.Unlock()
	b.killDispatcher <- true
	b.shutdown <- true
	b.config.disconnect()
	return nil
}

func (b *baseTransport) isClosed() bool {
	b.closedMu.RLock()
	closed := b.closed
	b.closedMu.RUnlock()
	return closed
}

func (b *baseTransport) isSubscribed(channel string) bool {
	b.subscriptionsMu.RLock()
	defer b.subscriptionsMu.RUnlock()
	for _, sub := range b.subscriptions {
		if sub.channel == channel {
			return true
		}
	}
	return false
}

func (b *baseTransport) getSubscription(subID string) (*subscription, error) {
	b.subscriptionsMu.RLock()
	subscription, ok := b.subscriptions[subID]
	b.subscriptionsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("No subscription with id %s", subID)
	}
	return subscription, nil
}

// Return the matching subscription for the message channel if a match exists.
func (b *baseTransport) getSubscriptionForMessageChannel(messageChannel chan *Message) *subscription {
	b.subscriptionsMu.RLock()
	defer b.subscriptionsMu.RUnlock()
	for _, sub := range b.subscriptions {
		if sub.messages == messageChannel {
			return sub
		}
	}

	return nil
}

func (b *baseTransport) getChannelString(subscriptions []*Subscription) string {
	channels := make([]string, len(subscriptions))
	for i, sub := range subscriptions {
		channels[i] = sub.Channel
	}
	return strings.Join(channels, ",")
}

func (b *baseTransport) getMaximumTimeout(subscriptions []*Subscription) time.Duration {
	timeout := 0 * time.Second
	for _, sub := range subscriptions {
		if sub.Timeout > timeout {
			timeout = sub.Timeout
		}
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return timeout
}

// Creates subscriptions for the passed in message channel. Will re-use the
// refCount and refMu if there are already subscriptions for the channel.
func (b *baseTransport) addSubscriptionsToChannel(messageChannel chan *Message,
	subscriptions []*Subscription) {
	var (
		refCount *uint32
		refMu    = &sync.Mutex{}
	)

	// Check if channel exists in existing subscriptions and re-use the
	// refCount and refMu pointers.
	subscript := b.getSubscriptionForMessageChannel(messageChannel)

	if subscript != nil {
		// Grab the count and mutex pointers.
		refCount = subscript.refCount
		refMu = subscript.refMu

		// Lock the mutex and add our subscription count to the count pointer.
		refMu.Lock()
		(*refCount) += uint32(len(subscriptions))
		refMu.Unlock()
	} else {
		// No existing use of the channel so set the count to the length of our
		// subscriptions.
		rc := uint32(len(subscriptions))
		refCount = &rc
	}

	// Add the subscriptions to the subscription map for their qualified
	// channels within the subscription mutex.
	b.subscriptionsMu.Lock()
	for _, sub := range subscriptions {
		b.subscriptions[sub.id] = &subscription{
			id:       sub.id,
			channel:  sub.Channel,
			messages: messageChannel,
			refCount: refCount,
			refMu:    refMu,
		}
	}
	b.subscriptionsMu.Unlock()
}

// Create a new subscription for the vessel channel and go channel.
func (b *baseTransport) createNewSubscription(channel string, msgs chan *Message) *subscription {
	refCount := uint32(1)

	return &subscription{
		channel:  channel,
		messages: msgs,
		refCount: &refCount,
		refMu:    &sync.Mutex{},
	}
}

func (b *baseTransport) waitForAck(timeout time.Duration, ackChan <-chan *ack) error {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	select {
	case ack := <-ackChan:
		if !ack.success {
			return fmt.Errorf("Failed ack received")
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("Ack timed out")
	}
}

func (b *baseTransport) waitForSubscriptionAck(ackChan <-chan *ack,
	subscriptions []*Subscription) error {
	var subAck *ack
	select {
	case subAck = <-ackChan:
		if !subAck.success {
			return fmt.Errorf("Failed ack received")
		}
	case <-time.After(b.getMaximumTimeout(subscriptions)):
		return fmt.Errorf("Ack timed out")
	}

	subIDs, ok := subAck.headers[subscriptionHeader]
	if !ok {
		return fmt.Errorf("Subscription ack missing %s header", subscriptionHeader)
	}

	subMap := map[string]string{}
	for _, header := range subIDs {
		keyVal := strings.Split(header, "=")
		if len(keyVal) != 2 {
			return fmt.Errorf("Bad header protocol for %s header %s", subscriptionHeader, header)
		}
		key := keyVal[0]
		val := keyVal[1]
		subMap[key] = val
	}

	for _, sub := range subscriptions {
		if id, ok := subMap[sub.Channel]; !ok {
			return fmt.Errorf("Missing id for channel %s", sub.Channel)
		} else {
			sub.id = id
		}
	}

	return nil
}

func (b *baseTransport) unsubscribe(subscript *Subscription) error {
	id := subscript.id
	b.subscriptionsMu.Lock()
	defer b.subscriptionsMu.Unlock()

	internalSub, ok := b.subscriptions[id]
	if !ok {
		return fmt.Errorf("No subscription with id %s.", subscript.id)
	}
	internalSub.unsubscribe()
	delete(b.subscriptions, id)
	return nil
}

func (b *baseTransport) addAck(envelope *Envelope, ackChan chan<- *ack) {
	need := uint(1)
	if envelope.Type == directedMsg {
		need = uint(len(envelope.Receivers))
	}

	// TODO: Track *who* acked.
	ackContext := &ackContext{need: need, done: ackChan}
	b.acksMu.Lock()
	b.acks[envelope.ID] = ackContext
	b.acksMu.Unlock()
}

// RemoveAck removes the ack context for the specified id.
func (b *baseTransport) RemoveAck(id string) {
	b.acksMu.Lock()
	delete(b.acks, id)
	b.acksMu.Unlock()
}

// dispatchLoop is a waits for received envelopes and dispatches them on the
// appropriate channels. It should be called from a goroutine.
func (b *baseTransport) dispatchLoop() {
	for {
		if b.isClosed() {
			break
		}

		b.dispatch()
	}
}

// dispatch processes a received envelope.
func (b *baseTransport) dispatch() {
	var env *Envelope
	select {
	case env = <-b.envelopes:
	case <-b.killDispatcher:
		return
	}

	// Attempt to deduplicate and ignore envelopes we sent.
	if b.filter.TestAndAdd([]byte(env.ID)) || b.config.dropEcho(env) {
		b.pool.Put(env)
		return
	}

	switch env.Type {
	case publishMsg:
		headers := parseHeader(env.Headers)
		subIDs, ok := headers[subscriptionHeader]
		if !ok {
			log.Println("Publish message missing %s header", subscriptionHeader)
			break
		}

		if b.config.applyFilters(envelopeToMessage(env)) {
			// Don't propagate if filtered.
			break
		}

		b.subscriptionsMu.RLock()
		for _, id := range subIDs {
			if subscription, ok := b.subscriptions[id]; ok {
				select {
				case subscription.messages <- envelopeToMessage(env):
				default:
					log.Printf("Subscription message dropped on channel %s, backpressure\n",
						subscription.channel)
				}
			}
		}
		b.subscriptionsMu.RUnlock()
	case directedMsg:
		select {
		case b.messages <- envelopeToMessage(env):
			break
		default:
			log.Println("Client messages dropped, backpressure")
		}
	case ackMsg:
		b.processAck(env)
	case eventMsg:
		b.processEvent(env)
	case heartbeatMsg:
	default:
		log.Println("Invalid envelope type", env.Type)
	}

	b.pool.Put(env)
}

func (b *baseTransport) processAck(env *Envelope) {
	if len(env.Body) < 2 {
		log.Printf("Bad ack protocol: %+v", env)
		return
	}
	successFlag := env.Body[0]
	ackID := string(env.Body[1:])

	b.acksMu.Lock()
	if ackContext, ok := b.acks[ackID]; ok {
		ackContext.acks++
		if ackContext.acks >= ackContext.need {
			if successFlag == ackSuccess {
				ackContext.done <- &ack{true, parseHeader(env.Headers)}
			} else {
				ackContext.done <- &ack{false, nil}
			}
			delete(b.acks, ackID)
		}
	}
	b.acksMu.Unlock()
}

func (b *baseTransport) processEvent(env *Envelope) {
	if len(env.Body) < 1 {
		log.Printf("Bad event protocol: %+v", env)
		return
	}
	eventType := EventType(env.Body[0])
	if eventType == EventReadbackFinished {
		b.offsetManager.ClearReadbackOffsets()
	}

	var data []byte
	if len(env.Body) > 1 {
		data = make([]byte, len(env.Body)-1)
		copy(data, env.Body[1:])
	}
	b.config.emitEvent(Event{Type: eventType, Data: data})
}

var (
	id        = uuid.RandomUUID
	timestamp = time.Now
)
