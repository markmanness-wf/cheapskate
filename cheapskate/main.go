package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
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
		httpAddr = flag.String("listen-http", "127.0.0.1:8888", "HTTP listen address")
		quotes = flag.String("quotes", "", "Path to quotes file")
		svcName = flag.String("service-name", "cheapskate", "The name of this service")
		maxDelayFlag = flag.Int("max-delay", -1, "The max delay to use before responding to requests")
	)
	flag.Parse()

	maxDelay := 10
	if maxDelayFlag != nil && *maxDelayFlag >= 0 {
		maxDelay = *maxDelayFlag
	} else if d := os.Getenv("CHEAP_MAX_DELAY"); d != "" {
		if i, err := strconv.Atoi(d); err == nil && i >= 0 {
			maxDelay = i
		} else {
			fmt.Printf("Invalid CHEAP_MAX_DELAY <%s>: %s\n", d, err)
			os.Exit(1)
		}
	}

	cheap := NewCheapskate(*quotes, maxDelay)
	bang := make(chan error)
	svcCnt := 0

	if (natsAddr != nil && *natsAddr != "") || os.Getenv("MSG_URL") != "" {
		fmt.Printf("Connect to nats and listen\n")
		svcCnt++
		go func () {
			n := NewNatsServer(cheap, *svcName, *natsAddr)
			bang <- n.ListenAndServe()
		}()
	}
	if httpAddr != nil && *httpAddr != "" {
		fmt.Printf("Start http server on <%s>\n", *httpAddr)
		svcCnt++
		go func() {
			h := NewRestServer(cheap, *httpAddr)
			bang <- h.ListenAndServe()
		}()
	}
	if svcCnt == 0 {
		fmt.Printf("No --nats-addr, no --listen-http, no MSG_URL environment variable\n")
		os.Exit(1)
	}
	err := <-bang
	fmt.Printf("error running server: %s\n", err)
	os.Exit(1)
}

