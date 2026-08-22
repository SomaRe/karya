BINARY = karya
INSTALL_DIR = /usr/local/bin

build:
	go build -o $(BINARY) .

install: build
	sudo cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	rm $(BINARY)

clean:
	rm -f $(BINARY)
