package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/nats-io/nats"

	w_service "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api"
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
		addr   = flag.String("addr", nats.DefaultURL, "NATS address")
		topic  = flag.String("topic", "stingy", "NATS topic to connect to")
		info   = flag.Bool("info", false, "Call the info method")
		ping   = flag.Bool("ping", false, "Call the ping method")
		health = flag.Bool("health", false, "Call the health method")
	)
	flag.Parse()

	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	fprotocolFactory := frugal.NewFProtocolFactory(protocolFactory)
	ftransportFactory := frugal.NewFMuxTransportFactory(5)

	natsOptions := nats.DefaultOptions

	if *addr != "" {
		natsOptions.Servers = []string{*addr}
	}
	conn, err := natsOptions.Connect()
	if err != nil {
		fmt.Printf("Error connecting to gnatsd: <%s>\n", err)
		os.Exit(1)
	}

	transport := frugal.NewNatsServiceTTransport(conn, *topic, 5*time.Second, 2)
	ftransport := ftransportFactory.GetTransport(transport)

	defer ftransport.Close()
	if err := ftransport.Open(); err != nil {
		fmt.Printf("Error opening frugal transport: <%s>\n", err)
		os.Exit(1)
	}

	client := w_service.NewFBaseServiceClient(ftransport, fprotocolFactory)

	err = nil
	if *health {
		err = checkHealth(client)
	} else if *info {
		err = checkInfo(client)
	} else if *ping {
		err = checkPing(client)
	} else {
		err = checkHealth(client)
	}
	if err != nil {
		fmt.Printf("Error connecting to service: %s\n", err)
		os.Exit(1)
	}
	return
}

func checkHealth(client *w_service.FBaseServiceClient) error {
	res, err := client.CheckServiceHealth(frugal.NewFContext(""))
	if err != nil {
		fmt.Printf("Failed to check health: %s\n", err)
		return err
	}
	fmt.Printf("Checked Health!!!\n")
	fmt.Printf("Version: <%s>\n", res.Version)
	fmt.Printf("Status: <%s>\n", res.Status)
	fmt.Printf("Message: <%s>\n", res.Message)

	if l := len(res.Metadata); l > 0 {
		fmt.Printf("Metadata: (%d)\n", l)
		for k, v := range(res.Metadata) {
			fmt.Printf("    <%s> => <%s>\n", k, v)
		}
	}
	return nil
}

func checkInfo(client *w_service.FBaseServiceClient) error {
	res, err := client.GetInfo(frugal.NewFContext(""))
	if err != nil {
		fmt.Printf("Failed to get info: %s\n", err)
		return err
	}
	fmt.Printf("Service info checked")
	fmt.Printf("Name: <%s>\n", res.Name)
	fmt.Printf("Version: <%s>\n", res.Version)
	fmt.Printf("Repo: <%s>\n", res.Repo)

	if res.ActiveRequests != nil {
		fmt.Printf("Active Requests: <%d>\n", *res.ActiveRequests)
	}
	if l := len(res.Metadata); l > 0 {
		fmt.Printf("Metadata: (%d)\n", l)
		for k, v := range(res.Metadata) {
			fmt.Printf("    <%s> => <%s>\n", k, v)
		}
	}
	return nil
}

func checkPing(client *w_service.FBaseServiceClient) error {
	err := client.Ping(frugal.NewFContext(""))
	if err != nil {
		fmt.Printf("Failed to ping service: %s\n", err)
		return err
	}
	fmt.Printf("Pinged service successfully\n")
	return nil
}
