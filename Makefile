project_name = nethadone
image_name = atomic77/nethadone:latest

NTH_WAN ?= eth0
NTH_LAN ?= lan0


help: ## This help dialog.
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

run-local: ## Run the app locally
	go run ./cmd/nethadone.go

dev:
	fiber dev -t cmd/nethadone.go

requirements: ## Generate go.mod & go.sum files
	go mod tidy

clean-packages: ## Clean packages
	go clean -modcache

up: ## Run the project in a local container
	make up-silent
	make shell

run-root: 
	go build -o build/ ./cmd/nethadone.go 
	sudo build/nethadone -lan-interface $(NTH_LAN) -wan-interface $(NTH_WAN) -config-file config/nethadone.yml
all:
	GOARCH=arm GOOS=linux go build -o build/nethadone-arm-linux ./cmd/nethadone.go 
	GOARCH=arm64 GOOS=linux go build -o build/nethadone-arm64-linux ./cmd/nethadone.go 
	GOARCH=amd64 GOOS=linux go build -o build/nethadone-amd64-linux ./cmd/nethadone.go 

build:
	go build ./cmd/nethadone.go

build-docker: ## Generate docker image
	docker build -t $(image_name) .

build-no-cache: ## Generate docker image with no cache
	docker build --no-cache -t $(image_name) .

up-silent: ## Run local container in background
	make delete-container-if-exist
	docker run -d -p 3000:3000 --name $(project_name) $(image_name) ./app

up-silent-prefork: ## Run local container in background with prefork
	make delete-container-if-exist
	docker run -d -p 3000:3000 --name $(project_name) $(image_name) ./app -prod

delete-container-if-exist: ## Delete container if it exists
	docker stop $(project_name) || true && docker rm $(project_name) || true

shell: ## Run interactive shell in the container
	docker exec -it $(project_name) /bin/sh

stop: ## Stop the container
	docker stop $(project_name)

start: ## Start the container
	docker start $(project_name)

### BPF related - with qdisc creation now built in, this is hopefully 
### deprecated
clean-tc:
	sudo tc filter del dev eth0 ingress
	sudo tc filter del dev eth1 ingress
	sudo tc filter del dev eth0 egress
	sudo tc filter del dev eth1 egress
