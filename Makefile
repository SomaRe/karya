BINARY = karya
INSTALL_DIR = /usr/local/bin

.PHONY: build install test clean

build:
	go build -o $(BINARY) .

install: build
	sudo install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
