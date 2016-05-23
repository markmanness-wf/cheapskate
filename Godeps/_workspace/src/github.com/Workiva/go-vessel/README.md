# go-vessel
[![codecov.io](https://codecov.workiva.net/github/Workiva/go-vessel/coverage.svg?token=7SFYthGn20&branch=master)](https://codecov.workiva.net/github/Workiva/go-vessel?branch=master)

Go client library for [Vessel](https://github.com/Workiva/vessel.git) messaging.

Two transports are currently supported: HTTP and TCP.

## Installation

```
go get github.com/Workiva/go-vessel
```

## Testing
*Running the unit tests only*

```
make unit
```

*Running the full integration tests*

This assumes that you have the [Vessel](https://github.com/Workiva/vessel) 
server installed in your go path

```
make integrations
```

## Usage

```go
import (
	"fmt"
	"strconv"
	"time"
	"github.com/Workiva/go-vessel"
)

clientID := "my-client-id"
vess, err := vessel.New([]string{"http://localhost:8000/vessel"}, vessel.NewConfig(clientID))
if err != nil {
	panic(err)
}

if err := vess.Dial(); err != nil {
	panic(err)
}

subscription := &vessel.Subscription{Channel: "foo"}
subscriptionC, err := vess.Subscribe(subscription)
if err != nil {
	panic(err)
}

go func() {
	for {
		select {
		case subscriptionMsg := <-subscriptionC:
			fmt.Println("foo recv", string(subscriptionMsg.Body))
		case directedMsg := <-vess.Messages():
			fmt.Println("directed recv", string(directedMsg.Body))
		case event := <-vess.Events():
			if event.Type == vessel.EventDisconnected {
				return
			}
		}
	}
}()

// Send a fire-and-forget directed message.
vess.Send([]byte("hello!"), []string{"some-client"}, vessel.Options{})

// Send a durable directed message acked by receiver.
resultC, errorC := vess.Send([]byte("ack this"), []string{"some-client"}, vessel.Options{
	QoS: vessel.AtLeastOnce,
	Retries: 3,
	RetryBackoff: true,
	AckTimeout: 1 * time.Second,
})

select {
case id := <-resultC:
    fmt.Println("acked", id)
case <-errorC:
    fmt.Println("not acked")
}

// Publish to a channel.
for i := 0; i < 10; i++ {
	vess.Publish([]byte(strconv.Itoa(i)), subscription, vessel.Options{})
	time.Sleep(time.Millisecond)
}

// Publish a durable message to a channel acked by vessel.
resultC, errorC = vess.Publish([]byte("ack me"), subscription,
    vessel.Options{QoS: vessel.AtLeastOnce})

select {
case id := <-resultC:
    fmt.Println("acked", id)
case <-errorC:
    fmt.Println("not acked")
}

vess.Unsubscribe(subscription)
vess.Close()
```

### Authentication

The http client is exported on the `vessel.Config`. Therefore, authentication with vessel may be handled by explicitly setting the http client. Below is an example that uses the `oauth2` JWT client.

```go
import (
	"ioutil"
	"github.com/Workiva/go-vessel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

clientID := "serviceAccountClientID"

key, err := ioutil.ReadFile("/.keys/private.pem"))
if err != nil {
	panic(err)
}

jwtCfg := &jwt.Config{
	Email: clientID,
	PrivateKey: key,
	Scopes: []string{"vessel|r", "vessel|w"},
	TokenURL: "http://localhost:8080/iam/oauth2/v1.0/token",
}
vesselCfg := &vessel.Config{
	ClientID: clientID,
	HTTPClient: jwtCfg.Client(oauth2.NoContext),
}
vess, err := vessel.New([]string{"http://localhost:8000/vessel"}, vesselCfg)
```

### TLS

Connecting to Vessel over TLS requires injecting a `tls.Config` into an `http.Transport` and into the Vessel transport.

```go
import (
	"crypto/tls"
	"net/http"
	"github.com/Workiva/go-vessel"
)

clientID := "abc"
config := vessel.NewConfig(clientID)
tlsConfig := &tls.Config{InsecureSkipVerify: true} // Use a more secure config
config.Options = map[vessel.Option]interface{}{vessel.OptionTLSConfig: tlsConfig}
tr := &http.Transport{TLSClientConfig: tlsConfig}
config.HTTPClient = &http.Client{Transport: tr}
vess, err := vessel.New([]string{"https://localhost:8000/vessel"}, config)
```
