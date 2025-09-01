.PHONY: build run tidy test

APP=surisink

build:
	GO111MODULE=on go build -o bin/$(APP) ./cmd/surisink

run:
	CONFIG_PATH=./configs/config.yaml go run ./cmd/surisink

tidy:
	go mod tidy

test:
	go test ./...
