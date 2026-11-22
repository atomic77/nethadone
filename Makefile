NTH_WAN ?= eth0
NTH_LAN ?= lan0
# Release name shown on the status page; CI passes the release tag
VERSION ?= dev
GOBUILD = go build -ldflags "-X github.com/atomic77/nethadone/config.Version=$(VERSION)"


help: ## This help dialog.
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

generate: ## Regenerate templ views (views/*_templ.go)
	go generate ./views

run-root: 
	$(GOBUILD) -o build/nethadone ./cmd
	sudo build/nethadone -lan-interface $(NTH_LAN) -wan-interface $(NTH_WAN) -config-file config/nethadone.yml
.PHONY: all build run-root generate test
all:
	GOARCH=arm GOOS=linux $(GOBUILD) -o build/nethadone-arm-linux ./cmd
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o build/nethadone-arm64-linux ./cmd
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o build/nethadone-amd64-linux ./cmd

build:
	$(GOBUILD) -o nethadone ./cmd

### Tests: see doc/development.md
KERNEL ?= v6.6.40

test: ## Unit tests (no root, no kernel features)
	go vet ./...
	go test ./...

test-bpf: ## Run the BPF programs on synthetic packets (sudo; attaches only to throwaway dummy interfaces)
	go test -exec sudo ./handlers -run BPF -v

test-vm: ## BPF tests in a virtme-ng VM booting upstream kernel $(KERNEL)
	go test -c -o build/handlers.test ./handlers
	vng -r $(KERNEL) --memory 2G --user root -- ./build/handlers.test -test.run BPF -test.v

lab-up: ## Namespaced router lab (scripts/netns-lab.sh); then lab-run, lab-traffic, lab-down
	sudo scripts/netns-lab.sh up
lab-run:
	sudo scripts/netns-lab.sh run
lab-traffic:
	sudo scripts/netns-lab.sh traffic
lab-down:
	sudo scripts/netns-lab.sh down
