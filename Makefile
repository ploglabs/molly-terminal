BINARY   := molly
CMD      := ./cmd/molly
GO       := go
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"
BUILD_DIR := bin

.PHONY: build install clean test lint fmt vet check cross-build

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)

install: build
	$(GO) install $(LDFLAGS) $(CMD)

clean:
	rm -rf $(BUILD_DIR)
	$(GO) clean

test:
	$(GO) test ./...

lint: vet fmt

vet:
	$(GO) vet ./...

fmt:
	$(GO)fmt -w .

check: vet test

cross-build:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64   $(CMD)
	GOOS=linux   GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64   $(CMD)
	GOOS=darwin  GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=darwin  GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(CMD)
	GOOS=windows GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-arm64.exe $(CMD)