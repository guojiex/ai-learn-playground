PYTHON ?= python3
NPM ?= npm

.PHONY: install-python install-web build-web dev-web test test-go test-python test-web run run-dev

install-python:
	cd python && $(PYTHON) -m pip install -r requirements.txt

install-web:
	cd web-studio && $(NPM) install

build-web:
	cd web-studio && $(NPM) run build

dev-web:
	cd web-studio && $(NPM) run dev

test: test-go test-python test-web

test-go:
	cd go && go test ./...

test-python:
	cd python && $(PYTHON) -m pytest

test-web:
	cd web-studio && $(NPM) test --silent

run:
	cd go && go run ./cmd/studio

# 开发：同时使用 Vite dev server。搭配 `make dev-web` 使用。
run-dev:
	cd go && AFFILIATE_STUDIO_DEV=1 go run ./cmd/studio
