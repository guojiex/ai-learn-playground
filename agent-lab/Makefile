# agent-lab Makefile
#
# 不依赖 Windows 上的 make: 等价 PowerShell 脚本见 tools/run.ps1.
#
# 常用目标:
#   make chat MSG="你好"         一次性聊一句 (流式)
#   make chat-once MSG="你好"    一次性聊一句 (非流式)
#   make fake                    在前台跑 fake-openai server (Ctrl-C 退出)
#   make web                     在前台跑 cmd/web (Ctrl-C 退出, 默认 127.0.0.1:8090)
#   make build                   构建所有二进制
#   make test                    跑全量单测
#   make vet                     go vet
#   make fmt                     gofmt -w

SHELL := /bin/sh

# 默认指向 fake-openai 端口, 真模型起来后覆盖即可:
#   OPENAI_BASE_URL=http://127.0.0.1:8080/v1 make chat MSG=...
OPENAI_BASE_URL ?= http://127.0.0.1:18080/v1
OPENAI_API_KEY  ?= sk-local
AGENTLAB_PROFILE ?= L
MSG ?= 你好, 我是新来的卖家, 请你介绍一下自己

ENV := OPENAI_BASE_URL=$(OPENAI_BASE_URL) OPENAI_API_KEY=$(OPENAI_API_KEY) AGENTLAB_PROFILE=$(AGENTLAB_PROFILE)
GO  ?= go

.PHONY: chat chat-once fake web build test vet fmt clean

chat:
	@$(ENV) $(GO) run ./cmd/chat -m "$(MSG)"

chat-once:
	@$(ENV) $(GO) run ./cmd/chat -m "$(MSG)" -no-stream

fake:
	@$(GO) run ./scripts/fake-openai

web:
	@$(ENV) $(GO) run ./cmd/web

build:
	$(GO) build ./...

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

clean:
	$(GO) clean
