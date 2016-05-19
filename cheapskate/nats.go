package main

import (
	"log"
	"github.com/Workiva/frugal/lib/go"
	"github.com/Workiva/messaging-sdk/lib/go/sdk"
	"github.com/danielrowles-wf/cheapskate/gen-go/stingy"
	w_model "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api_model"
)

type cheapNats struct {
	handler  *Cheapskate
	svcName  string
	natsAddr string
}

func NewNatsServer(handler *Cheapskate, svcName, natsAddr string) *cheapNats {
	return &cheapNats{
		handler:  handler,
		svcName:  svcName,
		natsAddr: natsAddr,
	}
}

func (n *cheapNats) ListenAndServe() error {
	options := sdk.NewOptions(n.svcName)
	if n.natsAddr != "" {
		options.NATSConfig.Servers = []string{n.natsAddr}
	}

	client := sdk.New(options)
	if err := client.Open(); err != nil {
		log.Printf("Failed to connect to NATS: %s", err)
		return err
	}
	defer client.Close()

	processor := stingy.NewFStingyServiceProcessor(n)
	service := sdk.Service(n.svcName)
	server, err := client.ProvideServer(service, processor)
	if err != nil {
		log.Printf("Failed to create a server: %s", err)
		return err
	}
	return server.Serve()
}

func (n *cheapNats) Ping(ctx *frugal.FContext) error {
	return n.handler.Ping(ctx.CorrelationID())
}

func (n *cheapNats) GetInfo(ctx *frugal.FContext) (*w_model.Info, error) {
	return n.handler.GetInfo(ctx.CorrelationID())
}

func (n *cheapNats) CheckServiceHealth(ctx *frugal.FContext) (*w_model.ServiceHealthStatus, error) {
	return n.handler.CheckServiceHealth(ctx.CorrelationID())
}

func (n *cheapNats) GetQuote(ctx *frugal.FContext) (string, error) {
	return n.handler.GetQuote(ctx.CorrelationID())
}
