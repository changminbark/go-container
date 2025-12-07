BINARY = gocontainer
ROOTLESS = rootless_container

all: build

build: container rootless

container:
	go build -o $(BINARY) main.go

install:
	go mod download

run:
	sudo ./$(BINARY) run /bin/bash

rootless:
	go build -o $(ROOTLESS) rootless/rootless.go

clean:
	rm -f $(BINARY) $(ROOTLESS)

.PHONY: all build container rootless install run clean