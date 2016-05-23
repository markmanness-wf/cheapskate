//go:generate msgp
package vessel

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/tinylib/msgp/msgp"
)

type MessageType uint8

// QoS indicates the quality-of-service level for a message. This determines
// the delivery guarantees.
type QoS int8

type Envelopes []*Envelope

const (
	// messageType constants
	publishMsg MessageType = iota + 1
	directedMsg
	ackMsg
	subscriptionMsg
	eventMsg
	heartbeatMsg

	// Ack message constants
	ackFailed  = byte(0)
	ackSuccess = byte(1)

	// Subscription message constants
	subscribe   = byte(0)
	unsubscribe = byte(1)
)

type marshaler interface {
	Marshal(*Envelope) ([]byte, error)
	MarshalArray([]*Envelope) ([]byte, error)
	Unmarshal([]byte) (*Envelope, error)
	UnmarshalArray([]byte) ([]*Envelope, error)
	MarshalPollRequest(*PollRequest) ([]byte, error)
	UnmarshalPollResponse([]byte) (*PollResponse, error)
}

func newMarshaler(config *Config) (marshaler, error) {
	switch config.ContentType {
	case MsgpackContent:
		return &msgpackMarshaler{}, nil
	default:
		return &jsonMarshaler{}, nil
	}
}

type jsonMarshaler struct{}

func (j *jsonMarshaler) Marshal(envelope *Envelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func (j *jsonMarshaler) MarshalArray(envelopes []*Envelope) ([]byte, error) {
	return json.Marshal(envelopes)
}

func (j *jsonMarshaler) Unmarshal(data []byte) (*Envelope, error) {
	var envelope Envelope
	err := json.Unmarshal(data, &envelope)
	return &envelope, err
}

func (j *jsonMarshaler) UnmarshalArray(data []byte) ([]*Envelope, error) {
	var envelopes []*Envelope
	err := json.Unmarshal(data, &envelopes)
	return envelopes, err
}

func (j *jsonMarshaler) MarshalPollRequest(pollRequest *PollRequest) ([]byte, error) {
	return json.Marshal(pollRequest)
}

func (j *jsonMarshaler) UnmarshalPollResponse(data []byte) (*PollResponse, error) {
	var pollResponse PollResponse
	err := json.Unmarshal(data, &pollResponse)
	return &pollResponse, err
}

type msgpackMarshaler struct{}

func (m *msgpackMarshaler) Marshal(envelope *Envelope) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := msgp.Encode(buf, envelope)
	return buf.Bytes(), err
}

func (m *msgpackMarshaler) MarshalArray(envelopes []*Envelope) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := msgp.Encode(buf, Envelopes(envelopes))
	return buf.Bytes(), err
}

func (m *msgpackMarshaler) Unmarshal(data []byte) (*Envelope, error) {
	var envelope Envelope
	buf := bytes.NewBuffer(data)
	err := msgp.Decode(buf, &envelope)
	return &envelope, err
}

func (m *msgpackMarshaler) UnmarshalArray(data []byte) ([]*Envelope, error) {
	var envelopes Envelopes
	buf := bytes.NewBuffer(data)
	err := msgp.Decode(buf, &envelopes)
	return []*Envelope(envelopes), err
}

func (m *msgpackMarshaler) MarshalPollRequest(pollRequest *PollRequest) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := msgp.Encode(buf, pollRequest)
	return buf.Bytes(), err
}

func (m *msgpackMarshaler) UnmarshalPollResponse(data []byte) (*PollResponse, error) {
	var pollResponse PollResponse
	buf := bytes.NewBuffer(data)
	err := msgp.Decode(buf, &pollResponse)
	return &pollResponse, err
}

// Message is a directed message received from a vessel.
type Message struct {
	ID        string      `msg:"-"`
	Channel   string      `msg:"-"`
	Timestamp time.Time   `msg:"-"`
	Receivers []string    `msg:"-"`
	Body      []byte      `msg:"-"`
	QoS       QoS         `msg:"-"`
	Origin    string      `msg:"-"`
	Header    http.Header `msg:"-"`
}

// Copy returns a deep copy of the Message.
func (m *Message) Copy() *Message {
	receivers := make([]string, len(m.Receivers))
	copy(receivers, m.Receivers)
	body := make([]byte, len(m.Body))
	copy(body, m.Body)
	header := make(http.Header, len(m.Header))
	for key, value := range m.Header {
		header[key] = value
	}
	message := &Message{
		ID:        m.ID,
		Channel:   m.Channel,
		Timestamp: m.Timestamp,
		Receivers: receivers,
		Body:      body,
		QoS:       m.QoS,
		Origin:    m.Origin,
		Header:    header,
	}
	return message
}

// Envelope is an implementation detail of the vessel message protocol. It
// should not be used directly.
type Envelope struct {
	Type      MessageType `json:"type",msg:"type"`
	ID        string      `json:"id",msg:"id"`
	Vector    []string    `json:"vector",msg:"vector"`
	Channel   string      `json:"channel",msg:"channel"`
	Timestamp int64       `json:"timestamp",msg:"timestamp"`
	QoS       QoS         `json:"qos",msg:"qos"`
	TTL       int64       `json:"ttl",msg:"ttl"`
	Receivers []string    `json:"receivers",msg"receivers"`
	Body      []byte      `json:"body",msg:"body"`
	Headers   string      `json:"headers,omitempty",msg:"headers,omitempty"`
	Partition int32       `json:"partition",msg:"partition"`
	Offset    int64       `json:"offset",msg:"offset"`
}

func envelopeBodySize(env interface{}) uint {
	return uint(len(env.(*Envelope).Body))
}

func (e *Envelope) origin() string {
	if len(e.Vector) == 0 {
		return ""
	}
	return e.Vector[0]
}

func envelopeToMessage(envelope *Envelope) *Message {
	receivers := make([]string, len(envelope.Receivers))
	copy(receivers, envelope.Receivers)
	body := make([]byte, len(envelope.Body))
	copy(body, envelope.Body)
	return &Message{
		ID:        envelope.ID,
		Channel:   envelope.Channel,
		Timestamp: time.Unix(envelope.Timestamp, 0),
		Receivers: receivers,
		Body:      body,
		QoS:       envelope.QoS,
		Origin:    envelope.origin(),
		Header:    parseHeader(envelope.Headers),
	}
}

func parseHeader(headers string) http.Header {
	reader := bufio.NewReader(strings.NewReader(headers + "\r\n"))
	tp := textproto.NewReader(reader)

	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		log.Println("Unable to parse headers", err)
		return nil
	}

	return http.Header(mimeHeader)
}

// This is an implementation detail and shouldn't be used externally,
// this is exported so msgp can generate code for it.
// PollRequest represents a request made to the "/v1/poll" endpoint
type PollRequest struct {
	PartitionOffsets map[string]int64 `json:"partition_offsets",msg:"partition_offsets"`
	Count            uint32           `json:"count",msg:"count"`
}

// This is an implementation detail and shouldn't be used externally,
// this is exported so msgp can generate code for it.
// PollResponse represents a the response to a poll sent to the "/v1/poll" endpoint
type PollResponse struct {
	PartitionOffsets map[string]int64 `json:"partition_offsets",msg:"partition_offsets"`
	Envelopes        []*Envelope      `json:"envelopes",msg:"envelopes"`
}
