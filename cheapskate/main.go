package main

import (
	"flag"
	"fmt"
	"os"
)

func Usage() {
	fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, "\n")
}

func main() {
	flag.Usage = Usage
	var (
		natsAddr = flag.String("nats-addr", "", "NATS address")
		httpAddr = flag.String("listen-http", "", "HTTP listen address")
		quotes = flag.String("quotes", "", "Path to quotes file")
		svcName = flag.String("service-name", "cheapskate", "The name of this service")
	)
	flag.Parse()

	cheap := NewCheapskate(*quotes)

	bang := make(chan error)

	go func () {
		n := NewNatsServer(cheap, *svcName, *natsAddr)
		bang <- n.ListenAndServe()
	}()

	if *httpAddr != "" {
		go func() {
			h := NewRestServer(cheap, *httpAddr)
			bang <- h.ListenAndServe()
		}()
	}

	err := <-bang
	fmt.Println("error running server:", err)
	os.Exit(1)
}

