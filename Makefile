.PHONY: all build clean test config

# Output directory
BIN_DIR := bin
CONFIG_DIR := config

# Executables
ifeq ($(OS),Windows_NT)
    EXE_EXT := .exe
else
    EXE_EXT :=
endif

SSH_EXECUTOR := $(BIN_DIR)/ssh-executor$(EXE_EXT)
CREDENTIAL_TOOL := $(BIN_DIR)/credential-tool$(EXE_EXT)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Default target
all: build

# Build all executables and copy configs
build: prepare-dirs $(SSH_EXECUTOR) $(CREDENTIAL_TOOL) copy-config

# Prepare directories
prepare-dirs:
ifeq ($(OS),Windows_NT)
	@if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	@if not exist $(BIN_DIR)\config mkdir $(BIN_DIR)\config
else
	mkdir -p $(BIN_DIR)/config
endif

# Build the main SSH executor
$(SSH_EXECUTOR):
	$(GOBUILD) -o $(SSH_EXECUTOR) ./cmd/main.go
ifeq ($(OS),Windows_NT)
	@echo Built $(SSH_EXECUTOR)
else
	chmod +x $(SSH_EXECUTOR)
endif

# Build the credential tool
$(CREDENTIAL_TOOL):
	$(GOBUILD) -o $(CREDENTIAL_TOOL) ./cmd/credential/main.go
ifeq ($(OS),Windows_NT)
	@echo Built $(CREDENTIAL_TOOL)
else
	chmod +x $(CREDENTIAL_TOOL)
endif

# Copy config files
copy-config:
ifeq ($(OS),Windows_NT)
	@if exist $(CONFIG_DIR) xcopy /E /I /Y $(CONFIG_DIR) $(BIN_DIR)\config
else
	cp -R $(CONFIG_DIR)/* $(BIN_DIR)/config/
endif

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
ifeq ($(OS),Windows_NT)
	@if exist $(BIN_DIR) rmdir /s /q $(BIN_DIR)
else
	rm -rf $(BIN_DIR)
endif

# Update dependencies
deps:
	$(GOMOD) tidy

# Install the tools to $GOPATH/bin
install: build
ifeq ($(OS),Windows_NT)
	copy $(SSH_EXECUTOR) "$(GOPATH)\bin\"
	copy $(CREDENTIAL_TOOL) "$(GOPATH)\bin\"
else
	cp $(SSH_EXECUTOR) "$(GOPATH)/bin/"
	cp $(CREDENTIAL_TOOL) "$(GOPATH)/bin/"
endif 