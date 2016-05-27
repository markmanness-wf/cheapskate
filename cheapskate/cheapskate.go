package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"
	w_model "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api_model"
)

type Cheapskate struct {
	quotes    []string
	maxDelay  int
}

func NewCheapskate(fortune string, maxDelay int) *Cheapskate {
	rand.Seed(time.Now().UnixNano())

	cs := &Cheapskate{
		maxDelay: maxDelay,
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

func (h *Cheapskate) Ping(cid string) error {
	log.Printf("Someone called Ping()")
	return nil
}

func (h *Cheapskate) GetInfo(cid string) (*w_model.Info, error) {
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

func (h *Cheapskate) CheckServiceHealth(cid string) (*w_model.ServiceHealthStatus, error) {
	log.Printf("Someone called CheckServiceHealth()")

	res := rand.Intn(5)
	dur := rand.Intn(h.maxDelay)

	time.Sleep(time.Duration(dur) * time.Second)

	out := w_model.NewServiceHealthStatus()
	out.Message = fmt.Sprintf("All Ur <%s> Are Belong To DevOps", cid)
	out.Metadata = map[string]string{
		"pies": "tasty",
		"beer": "yummy",
		"corr": cid,
		"sleep": strconv.Itoa(dur),
	}
	if res == 0 {
		out.Status = w_model.HealthCondition_PASS
	} else if res == 1 {
		out.Status = w_model.HealthCondition_WARN
	} else if res == 2 {
		out.Status = w_model.HealthCondition_FAIL
	} else if res == 3 {
		out.Status = w_model.HealthCondition_UNKNOWN
	} else {
		return nil, &w_model.BaseError{
			Code:    314,
			Message: "Borky Borky Bork",
		}
		// return nil, errors.New("Borky Borky Bork")
	}

	if len(h.quotes) > 0 {
		out.Metadata["quote"] = h.quotes[rand.Intn(len(h.quotes))]
	}
	return out, nil
}

func (h *Cheapskate) GetQuote(cid string) (string, error) {
	if len(h.quotes) == 0 {
		return "All Ur Quote Are Belong To DevOps", nil
	}
	idx := rand.Intn(len(h.quotes))
	return h.quotes[idx], nil
}
