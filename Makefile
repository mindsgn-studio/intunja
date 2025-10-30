run-server:
	go run ./cmd/server

run-client:
	go run ./cmd/client -remote="127.0.0.1:8080" -local="http://localhost:3000" -reconnect=5s -keepalive=10s -timeout=30s

build:
	go build -o bin/server ./cmd/server
	go build -o bin/client ./cmd/client