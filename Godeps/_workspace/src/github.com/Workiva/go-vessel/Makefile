SHELL := /bin/bash

.PHONY: unit integrations

unit:
	godep go test -v -short ./...

integrations:
	./scripts/run_tests_local.sh

test_w_vessel:
	./scripts/run_tests_local_w_vessel.sh
