APP := ember
BUILD_DIR := bin
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: build install clean

build:
	go build -o $(BUILD_DIR)/$(APP) .

install: build
	mkdir -p $(BINDIR)
	cp $(BUILD_DIR)/$(APP) $(BINDIR)/$(APP)
	@echo "Installed: $(BINDIR)/$(APP)"

clean:
	rm -rf $(BUILD_DIR)
