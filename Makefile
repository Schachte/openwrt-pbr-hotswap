
BINARY_NAME=pbr-vpn

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

BUILD_DIR=build
CMD_DIR=cmd/pbr-vpn

VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS=-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

ROUTER_HOST ?= 192.168.1.1
ROUTER_USER ?= root
ROUTER_PATH ?= /apps
ROUTER_PORT ?= 8080
SSH_KEY ?= ~/.ssh/openwrt
CONFIG_FILE ?= config.json

ifdef SSH_KEY
SSH_OPTS=-i $(SSH_KEY)
SCP_OPTS=-i $(SSH_KEY) -O
else
SSH_OPTS=
SCP_OPTS=-O
endif

TLS_DIR=tls
TLS_CERT=$(TLS_DIR)/cert.pem
TLS_KEY=$(TLS_DIR)/key.pem
ROUTER_TLS_PATH=/etc/pbr-vpn

.PHONY: all build build-linux-arm64 build-linux-amd64 build-all clean test deps \
        deploy deploy-amd64 health logs logs-f stop install-service help run \
        generate-certs

all: build

help:
	@echo "PBR-VPN Makefile"
	@echo ""
	@echo "Build targets:"
	@echo "  make build              - Build for current platform"
	@echo "  make build-linux-arm64  - Cross-compile for Linux ARM64 (most OpenWRT routers)"
	@echo "  make build-linux-amd64  - Cross-compile for Linux AMD64 (x86 routers)"
	@echo "  make build-all          - Build for all Linux platforms"
	@echo ""
	@echo "Development:"
	@echo "  make run                - Build and run locally (for testing)"
	@echo "  make test               - Run tests"
	@echo "  make deps               - Download dependencies"
	@echo "  make clean              - Remove build artifacts"
	@echo ""
	@echo "TLS:"
	@echo "  make generate-certs     - Generate self-signed TLS certificates"
	@echo ""
	@echo "Deployment:"
	@echo "  make deploy             - Build ARM64 and deploy to router (includes TLS certs)"
	@echo "  make deploy-amd64       - Build AMD64 and deploy to router"
	@echo "  make health             - Check if server is running on router"
	@echo "  make logs               - View logs from router"
	@echo "  make logs-f             - Follow logs from router"
	@echo "  make stop               - Stop server on router"
	@echo "  make install-service    - Install as OpenWRT init.d service"
	@echo ""
	@echo "Configuration (override with env vars or make VAR=value):"
	@echo "  ROUTER_HOST   = $(ROUTER_HOST)"
	@echo "  ROUTER_USER   = $(ROUTER_USER)"
	@echo "  ROUTER_PATH   = $(ROUTER_PATH)"
	@echo "  ROUTER_PORT   = $(ROUTER_PORT)"
	@echo "  SSH_KEY       = $(SSH_KEY) (optional)"
	@echo "  CONFIG_FILE   = $(CONFIG_FILE) (optional)"
	@echo ""
	@echo "Examples:"
	@echo "  make deploy ROUTER_HOST=192.168.1.1"
	@echo "  make deploy ROUTER_HOST=192.168.1.1 SSH_KEY=~/.ssh/router_key"
	@echo "  make health ROUTER_HOST=192.168.1.1"

deps:
	$(GOMOD) download
	$(GOMOD) tidy

build: deps
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux-arm64: deps
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64

build-linux-amd64: deps
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64

build-all: build-linux-arm64 build-linux-amd64

run: build
	$(BUILD_DIR)/$(BINARY_NAME) -listen :8080 -vpn-interface vpn_amsterdam \
		-dhcp-leases /tmp/test-dhcp.leases -ethers /tmp/test-ethers

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

generate-certs:
	@mkdir -p $(TLS_DIR)
	@if [ ! -f $(TLS_CERT) ] || [ ! -f $(TLS_KEY) ]; then \
		echo "Generating self-signed TLS certificate..."; \
		openssl req -x509 -newkey rsa:2048 \
			-keyout $(TLS_KEY) \
			-out $(TLS_CERT) \
			-days 365 -nodes \
			-subj "/CN=pbr-vpn" \
			-addext "subjectAltName=IP:$(ROUTER_HOST),IP:127.0.0.1"; \
		echo "Generated: $(TLS_CERT) and $(TLS_KEY)"; \
	else \
		echo "TLS certificates already exist in $(TLS_DIR)/"; \
	fi

deploy: build-linux-arm64 generate-certs
	@echo "Deploying to $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_PATH)..."
	@echo ""
	@# Create directories
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "mkdir -p $(ROUTER_PATH) $(ROUTER_TLS_PATH)"
	@# Stop existing service if running
	@echo "Stopping existing service (if running)..."
	-ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "killall $(BINARY_NAME) 2>/dev/null" || true
	@# Copy binary
	@echo "Copying binary..."
	scp $(SCP_OPTS) $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_PATH)/$(BINARY_NAME)
	@# Copy TLS certificates
	@echo "Copying TLS certificates..."
	scp $(SCP_OPTS) $(TLS_CERT) $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_TLS_PATH)/cert.pem
	scp $(SCP_OPTS) $(TLS_KEY) $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_TLS_PATH)/key.pem
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "chmod 600 $(ROUTER_TLS_PATH)/key.pem"
	@# Copy config if exists
	@if [ -f $(CONFIG_FILE) ]; then \
		echo "Copying $(CONFIG_FILE)..."; \
		scp $(SCP_OPTS) $(CONFIG_FILE) $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_PATH)/config.json; \
	fi
	@# Make executable
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "chmod +x $(ROUTER_PATH)/$(BINARY_NAME)"
	@# Start server with config if exists
	@echo "Starting server..."
	@if [ -f $(CONFIG_FILE) ]; then \
		ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "$(ROUTER_PATH)/$(BINARY_NAME) -config $(ROUTER_PATH)/config.json > $(ROUTER_PATH)/pbr-vpn.log 2>&1 &"; \
	else \
		ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "$(ROUTER_PATH)/$(BINARY_NAME) -listen :$(ROUTER_PORT) > $(ROUTER_PATH)/pbr-vpn.log 2>&1 &"; \
	fi
	@echo ""
	@echo "Waiting for server to start..."
	@sleep 3
	@$(MAKE) health --no-print-directory

deploy-amd64: build-linux-amd64
	@echo "Deploying AMD64 to $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_PATH)..."
	@echo ""
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "mkdir -p $(ROUTER_PATH)"
	-ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "killall $(BINARY_NAME) 2>/dev/null" || true
	scp $(SCP_OPTS) $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(ROUTER_USER)@$(ROUTER_HOST):$(ROUTER_PATH)/$(BINARY_NAME)
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "chmod +x $(ROUTER_PATH)/$(BINARY_NAME)"
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "$(ROUTER_PATH)/$(BINARY_NAME) -listen :$(ROUTER_PORT) > $(ROUTER_PATH)/pbr-vpn.log 2>&1 &"
	@sleep 2
	@$(MAKE) health --no-print-directory

health:
	@echo "Checking health at https://$(ROUTER_HOST):$(ROUTER_PORT)/health ..."
	@curl -sfk https://$(ROUTER_HOST):$(ROUTER_PORT)/health && echo "" && echo "Server is healthy!" || \
		(echo "Health check failed! Server may not be running." && exit 1)

logs:
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "cat $(ROUTER_PATH)/pbr-vpn.log"

logs-f:
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "tail -f $(ROUTER_PATH)/pbr-vpn.log"

stop:
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "killall $(BINARY_NAME) 2>/dev/null" || true
	@echo "Server stopped"

install-service: deploy
	@echo "Installing init.d service with auto-restart on router..."
	@ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "printf '%s\n' \
		'#!/bin/sh /etc/rc.common' \
		'' \
		'START=99' \
		'STOP=10' \
		'USE_PROCD=1' \
		'' \
		'PROG=$(ROUTER_PATH)/$(BINARY_NAME)' \
		'CONFIG=$(ROUTER_PATH)/config.json' \
		'' \
		'start_service() {' \
		'    procd_open_instance' \
		'    procd_set_param command \$$PROG -config \$$CONFIG' \
		'    procd_set_param respawn 3600 5 5' \
		'    procd_set_param stdout 1' \
		'    procd_set_param stderr 1' \
		'    procd_set_param pidfile /var/run/pbr-vpn.pid' \
		'    procd_close_instance' \
		'}' \
		'' \
		'stop_service() {' \
		'    killall $(BINARY_NAME) 2>/dev/null' \
		'}' > /etc/init.d/pbr-vpn"
	ssh $(SSH_OPTS) $(ROUTER_USER)@$(ROUTER_HOST) "chmod +x /etc/init.d/pbr-vpn && /etc/init.d/pbr-vpn enable && /etc/init.d/pbr-vpn restart"
	@sleep 2
	@$(MAKE) health --no-print-directory
	@echo ""
	@echo "Service installed with auto-start and auto-restart!"
	@echo "  - Starts on boot (S99)"
	@echo "  - Auto-restarts if crashed (respawn)"
	@echo ""
	@echo "Commands:"
	@echo "  /etc/init.d/pbr-vpn start|stop|restart"
	@echo "  /etc/init.d/pbr-vpn disable  (disable auto-start)"

STRIP_TOOL=scripts/stripcomments

$(STRIP_TOOL): scripts/stripcomments.go
	@echo "Building comment stripping tool..."
	$(GOBUILD) -o $(STRIP_TOOL) scripts/stripcomments.go

remove-comments: $(STRIP_TOOL)
	@echo "Removing all Go comments recursively..."
	@find . -type f -name '*.go' -not -path './vendor/*' | while read f; do \
		echo "Stripping comments from $$f"; \
		$(STRIP_TOOL) "$$f"; \
	done
	@echo "Done!"
