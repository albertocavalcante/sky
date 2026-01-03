.PHONY: build test lint format tidy gazelle

build:
	bazel build //cmd/...

test:
	bazel test //...

lint:
	bazel build //...

format:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

gazelle:
	bazel run //:gazelle

tidy:
	go mod tidy
