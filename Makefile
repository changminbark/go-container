BINARY = gocontainer

all: build

build:
	go build -o $(BINARY) main.go

run:
	sudo ./$(BINARY) run /bin/bash

clean:
	rm -f $(BINARY)