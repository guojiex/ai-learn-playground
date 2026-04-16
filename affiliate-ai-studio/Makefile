PYTHON ?= python3

.PHONY: install-python test test-go test-python run

install-python:
	cd python && $(PYTHON) -m pip install -r requirements.txt

test: test-go test-python

test-go:
	cd go && go test ./...

test-python:
	cd python && $(PYTHON) -m pytest

run:
	cd go && go run ./cmd/studio
