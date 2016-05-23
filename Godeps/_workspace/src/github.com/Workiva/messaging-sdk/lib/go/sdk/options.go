package sdk

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/Workiva/go-vessel"
	"github.com/nats-io/nats"
	"golang.org/x/oauth2/jwt"
)

// ThriftProtocol specifies a serialization protocol used by Frugal.
type ThriftProtocol string

const (
	ProtocolBinary  ThriftProtocol = "binary"
	ProtocolCompact ThriftProtocol = "compact"
	ProtocolJSON    ThriftProtocol = "json"

	defaultHeartbeatInterval   = 5 * time.Second
	defaultConnectTimeout      = 10 * time.Second
	defaultMaxMissedHeartbeats = 3
	defaultTransportBuffer     = 8192
)

// Options contains configuration settings for an sdk client.
type Options struct {
	NATSConfig           nats.Options
	ClientCertFilepath   string
	ClientCACertFilepath string
	ThriftProtocol       ThriftProtocol
	ThriftBufferSize     uint
	HeartbeatInterval    time.Duration
	MaxMissedHeartbeats  uint
	ConnectTimeout       time.Duration
	NumWorkers           uint
	VesselHosts          []string
	VesselConfig         vessel.Config
	VesselAuth           *jwt.Config
	IamTokenFetcher      IamTokenFetcher
}

// NewOptions creates a new Options for the given service client id.
func NewOptions(clientID string) Options {
	cfg := nats.DefaultOptions
	servers := MsgURL()
	if len(servers) > 0 {
		// If MSG_URL set, override any URL set in the config
		cfg.Url = ""
		cfg.Servers = servers
	} else if cfg.Url == "" {
		// If nothing is set, take the default url
		cfg.Url = nats.DefaultURL
	}
	cfg.Name = fmt.Sprintf("%s-%d", clientID, os.Getpid())

	return Options{
		NATSConfig:           cfg,
		ClientCertFilepath:   MsgCert(),
		ClientCACertFilepath: MsgCACert(),
		ThriftProtocol:       ProtocolBinary,
		ThriftBufferSize:     defaultTransportBuffer,
		HeartbeatInterval:    defaultHeartbeatInterval,
		MaxMissedHeartbeats:  defaultMaxMissedHeartbeats,
		ConnectTimeout:       defaultConnectTimeout,
		NumWorkers:           uint(runtime.NumCPU()),
		VesselConfig:         *vessel.NewConfig(clientID),
	}
}

func (o Options) newThriftProtocolFactory() (thrift.TProtocolFactory, error) {
	var protocolFactory thrift.TProtocolFactory
	switch o.ThriftProtocol {
	case ProtocolBinary:
		protocolFactory = thrift.NewTBinaryProtocolFactoryDefault()
	case ProtocolCompact:
		protocolFactory = thrift.NewTCompactProtocolFactory()
	case ProtocolJSON:
		protocolFactory = thrift.NewTJSONProtocolFactory()
	default:
		return nil, fmt.Errorf("sdk: invalid protocol specified: %s", o.ThriftProtocol)
	}
	return protocolFactory, nil
}

func (o Options) newThriftTransportFactory() thrift.TTransportFactory {
	var transportFactory thrift.TTransportFactory
	if int(o.ThriftBufferSize) > 0 {
		transportFactory = thrift.NewTBufferedTransportFactory(int(o.ThriftBufferSize))
	} else {
		transportFactory = thrift.NewTTransportFactory()
	}
	return transportFactory
}
