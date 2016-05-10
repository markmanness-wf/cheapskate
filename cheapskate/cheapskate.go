package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"math/rand"

	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/nats-io/nats"

	"github.com/danielrowles-wf/cheapskate/gen-go/stingy"
	// w_service "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api"
	w_model "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api_model"
	"github.com/Workiva/frugal/lib/go"
)

func Usage() {
	fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, "\n")
}

func main() {
	flag.Usage = Usage
	var (
		addr = flag.String("addr", "", "NATS address")
		quotes = flag.String("quotes", "", "Path to quotes file")
		topic = flag.String("topic", "", "NATS topic to listen on")
	)
	flag.Parse()

	handler := NewCheapskate(*quotes)

	if err := runServer(handler, *topic, *addr); err != nil {
		fmt.Println("error running server:", err)
		os.Exit(1)
	}
}

func runServer(handler *Cheapskate, topic string, natsAddr string) error {
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	fprotocolFactory := frugal.NewFProtocolFactory(protocolFactory)
	ftransportFactory := frugal.NewFMuxTransportFactory(5)

	natsOptions := nats.DefaultOptions

	if natsAddr != "" {
		natsOptions.Servers = []string{natsAddr}
	} else if msgUrl := os.Getenv("MSG_URL"); msgUrl != "" {
		natsOptions.Servers = []string{msgUrl}
	}

	// TODO - messaging sdk once MSG-109 is done will change how this is done
	if topic == "" {
		if e := os.Getenv("MSG_HEALTH_TOPIC"); e != "" {
			topic = e
		} else {
			topic = "stingy"
		}
	}

	conn, err := natsOptions.Connect()
	if err != nil {
		panic(err)
	}
	processor := stingy.NewFStingyServiceProcessor(handler)
	server := frugal.NewFNatsServerFactory(conn, topic, 20*time.Second, 2, frugal.NewFProcessorFactory(processor), ftransportFactory, fprotocolFactory)

	return server.Serve()
}

// Sever handler
//
type Cheapskate struct {
	quotes []string
}

func NewCheapskate(fortune string) *Cheapskate {
	cs := &Cheapskate{
		quotes: []string{
			"Never Eat Yellow Snow",
			"If you don't know what introspection is you need to take a long, hard look at yourself",
			"Never trust an atom. They make up everything.",
			"What's the difference between a 'hippo' and a 'Zippo'? One is really heavy, the other is a little lighter",
			"I took the shell off my racing snail, thinking it would make him run faster. If anything, it made him more sluggish.",
			"The past, the present, and the future walked into a bar. It was tense.",
			"What's large, grey, and doesn't matter? An irrelephant.",
			"Did you hear the one about the hungry clock? It went back four seconds.",
			"Someone stole my Microsoft Office and they're gonna pay. You have my Word.",
			"Did you hear about the three holes I dug in my garden? The ones that filled with water? Well, well, well...",
			"What do you call a fly without wings? A walk.",
			"Whats the difference between ignorance and apathy?  i dont know and i dont care",
		},
	}
	return cs
}

func (h *Cheapskate) Ping(ctx *frugal.FContext) error {
	log.Printf("Someone called Ping()")
	return nil
}

func (h *Cheapskate) GetInfo(ctx *frugal.FContext) (*w_model.Info, error) {
	log.Printf("Someone called GetInfo()")

	requests := int64(314)

	info := w_model.NewInfo()
	info.Name = "cheapskate"
	info.Version = "0.0.1"
	info.Repo = "git@github.com:danielrowles-wf/cheapskate.git"
	info.ActiveRequests = &requests
	info.Metadata = map[string]string{
		"pies": "tasty",
		"beer": "yummy",
	}
	return info, nil
}

func (h *Cheapskate) CheckServiceHealth(ctx *frugal.FContext) (*w_model.ServiceHealthStatus, error) {
	log.Printf("Someone called CheckServiceHealth()")

	cid := ctx.CorrelationID()
	out := w_model.NewServiceHealthStatus()
	out.Status = w_model.HealthCondition_PASS
	out.Message = fmt.Sprintf("All Ur <%s> Are Belong To DevOps", cid)
	out.Metadata = map[string]string{
		"pies": "tasty",
		"beer": "yummy",
		"corr": cid,
	}
	if len(h.quotes) > 0 {
		out.Metadata["quote"] = h.quotes[rand.Intn(len(h.quotes))]
	}
	return out, nil
}

func (h *Cheapskate) GetQuote(ctx *frugal.FContext) (string, error) {
	if len(h.quotes) == 0 {
		return "All Ur Quote Are Belong To DevOps", nil
	}
	idx := rand.Intn(len(h.quotes))
	return h.quotes[idx], nil
}
