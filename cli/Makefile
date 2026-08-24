.PHONY: build install test clean

build:
	go build -o bin/rootcauseway ./cmd/rootcauseway

install: build
	cp bin/rootcauseway /usr/local/bin/

test:
	go test ./...

clean:
	rm -rf bin/
