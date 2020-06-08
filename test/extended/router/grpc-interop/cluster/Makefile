export GO111MODULE=on

all:
	go build -o /dev/null client/client.go
	go build -o /dev/null server/server.go
	go mod verify
	go mod tidy
