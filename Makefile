run-server:
	go run ./cmd/server

run-client:
	go run ./cmd/client

build:
	go build -o bin/server ./cmd/server
	go build -o bin/client ./cmd/client