all: build
build:
	go build -o bin/server ./server
	go build -o bin/client ./client
	go build -o bin/ping ./ping
