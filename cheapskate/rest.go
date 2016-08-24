package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	w_model "github.com/danielrowles-wf/cheapskate/gen-go/workiva_frugal_api_model"
)

type cheapRest struct {
	handler    *Cheapskate
	listenAddr string
	timeout int
}

func NewRestServer(handler *Cheapskate, listenAddr string, timeout int) *cheapRest {
	return &cheapRest{
		handler:    handler,
		listenAddr: listenAddr,
		timeout: timeout,
	}
}

func (r *cheapRest) ListenAndServe() error {
	
	server := &http.Server{
		Addr:           r.listenAddr,
		Handler:        r,
		ReadTimeout:    timeout * time.Second,
		WriteTimeout:   timeout * time.Second,
		MaxHeaderBytes: 65535,
	}
	return server.ListenAndServe()
}

func (r *cheapRest) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	cid := req.Header.Get("X-WK-Correlation-Id")
	path := req.URL.Path
	if path == "/ping" {
		err := r.handler.Ping(cid)
		if err != nil {
			log.Printf("Failed to ping: %s", err)
			r.sendError(res)
			return
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("Pong Pong Pong Pong Pong Pong Pong Pong\n"))

		for name, vals := range(req.Header) {
			for _, val := range(vals) {
				res.Write([]byte(name))
				res.Write([]byte(": "))
				res.Write([]byte(val))
				res.Write([]byte("\n"))
			}
		}
		return
	} else if path == "/info" {
		info, err := r.handler.GetInfo(cid)
		if err != nil {
			log.Printf("Failed to get service info: %s", err)
			r.sendError(res)
			return
		}
		r.sendJson(res, 200, "application/json", info)
	} else if path == "/health" {
		health, err := r.handler.CheckServiceHealth(cid)
		if err != nil {
			log.Printf("Failed to check service health: %s", err)
			r.sendError(res)
			return
		}
		code := r.statusToResponseCode(health.Status)
		ctype := "application/x.workiva.health"
		r.sendJson(res, code, ctype, health)
	} else if path == "/quote" {
		quote, err := r.handler.GetQuote(cid)
		if err != nil {
			log.Printf("Failed to get quote: %s", err)
			r.sendError(res)
			return
		}
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(quote))
	} else {
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusNotFound)
		res.Write([]byte("Not found"))
	}
}

func (r *cheapRest) sendError(res http.ResponseWriter) {
	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusInternalServerError)
	res.Write([]byte("Bang\n"))
}

func (r *cheapRest) sendJson(res http.ResponseWriter, code int, ctype string, out interface{}) {
	blob, err := json.Marshal(out)
	if err != nil {
		log.Printf("Failed to serialize <%+v>: %s", out, err)
		r.sendError(res)
		return
	}
	res.Header().Set("Content-Type", ctype)
	res.WriteHeader(code)
	res.Write(blob)
}

func (r *cheapRest) statusToResponseCode(status w_model.HealthCondition) int {
	if status == w_model.HealthCondition_PASS {
		return 200
	} else if status == w_model.HealthCondition_WARN {
		return 429
	} else if status == w_model.HealthCondition_FAIL {
		return 503
	} else if status == w_model.HealthCondition_UNKNOWN {
		return 520
	} else {
		return 500
	} 
}
