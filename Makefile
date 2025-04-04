BINARY = gocontainer
ROOTLESS = rootless_container

all: build rootless

build:
	go build -o $(BINARY) main.go

run:
	sudo ./$(BINARY) run /bin/bash

rootless:
	go build -o $(ROOTLESS) rootless/rootless.go

clean:
	rm -f $(BINARY) $(ROOTLESS)

.PHONY: all build rootless run clean