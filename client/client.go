package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Workiva/frugal/lib/go"
	"github.com/Workiva/messaging-sdk/lib/go/sdk"
	w_service "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api"
)

func Usage() {
	fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, "\n")
}

func main() {
	flag.Usage = Usage
	var (
		natsAddr = flag.String("addr", "", "NATS address")
		svcName  = flag.String("service-name", "cheapskate", "Service name")
		info     = flag.Bool("info", false, "Call the info method")
		ping     = flag.Bool("ping", false, "Call the ping method")
		health   = flag.Bool("health", false, "Call the health method")
	)
	flag.Parse()

	options := sdk.NewOptions(*svcName)
	if *natsAddr != "" {
		options.NATSConfig.Servers = []string{*natsAddr}
	}

	client := sdk.New(options)
	if err := client.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to NATS: %s\n", err)
		os.Exit(1)
	}
	defer client.Close()

	service := sdk.Service(*svcName)
	transport, protocolFactory, err := client.ProvideClient(service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create NATS client: %s\n", err)
		os.Exit(1)
	}
	if err := transport.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open NATS transport: %s\n", err)
		os.Exit(1)
	}
	defer transport.Close()

	healthClient := w_service.NewFBaseServiceClient(transport, protocolFactory)

	err = nil
	if *health {
		err = checkHealth(healthClient)
	} else if *info {
		err = checkInfo(healthClient)
	} else if *ping {
		err = checkPing(healthClient)
	} else {
		err = checkHealth(healthClient)
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
