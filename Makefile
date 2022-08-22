#!make
.DEFAULT_GOAL 	= help

PROJECT			= hetzner-k3s
GIT_AUTHOR		= janosmiko
MAKEFILE		:= $(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))

# If .env file exists, import it to make and export the variables from it to the make targets.
-include .env
export $(shell sed 's/=.*//' $(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

help: ## Outputs this help screen
	@grep -E '(^[\/a-zA-Z0-9_-]+:.*?##.*$$)|(^##)' $(MAKEFILE) | awk 'BEGIN {FS = ":.*?## "}{printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

## —— Build Commands —————————————————————————————————————————————————————————
build: ## Build a snapshot of all binaries
	goreleaser build --rm-dist --snapshot

build-packages: ## Build a snapshot of all binaries create a package
	goreleaser release --rm-dist --auto-snapshot --skip-publish

release: ## Build and release everything (GITHUB_TOKEN has to be set)
	goreleaser release --rm-dist

## —— Run Commands —————————————————————————————————————————————————————————
create-cluster: ## Build and run the `create-cluster` subcommand
	go run cmd/hetzner-k3s/main.go create-cluster

delete-cluster: ## Build and run the `delete-cluster` subcommand
	go run cmd/hetzner-k3s/main.go delete-cluster

releases: ## Build and run the `releases` subcommand
	go run cmd/hetzner-k3s/main.go releases

## —— Go Commands -—————————————————————————————————————————————————————————
mod: ## Update Go Dependencies
	go mod tidy

lint: ## Lint Go Code
	golangci-lint run

lint-github: ## Lint Go Code in github format
	golangci-lint run --out-format=github-actions

test: ## Run Go tests
	go test -race ./... -v
