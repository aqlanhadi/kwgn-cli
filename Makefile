# Makefile for kwgn CLI

BINARY_NAME=kwgn
INSTALL_PATH=/usr/local/bin

.PHONY: all build install clean

all: build

build:
	go build -o $(BINARY_NAME) .

install: build
	mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME) ç