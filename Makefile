.PHONY: all go rs ts cpp java install clean

BIN_DIR := bin

all: go rs ts

# --- Go: MCP server + Go checker ---

GO_SOURCES := $(shell find go -name '*.go' 2>/dev/null)

$(BIN_DIR)/mcp-lint: $(GO_SOURCES)
	@echo "Building MCP server..."
	@mkdir -p $(BIN_DIR)
	cd go && go build -ldflags="-w -s" -o ../$(BIN_DIR)/mcp-lint ./cmd/mcp-lint

$(BIN_DIR)/mcp-lint-go: $(GO_SOURCES)
	@echo "Building Go checker..."
	@mkdir -p $(BIN_DIR)
	cd go && go build -ldflags="-w -s" -o ../$(BIN_DIR)/mcp-lint-go ./cmd/mcp-lint-go

go: $(BIN_DIR)/mcp-lint $(BIN_DIR)/mcp-lint-go

go-test:
	cd go && go test ./...

# --- Rust checker ---

RS_SOURCES := $(shell find rs/src -name '*.rs' 2>/dev/null) rs/Cargo.toml

$(BIN_DIR)/mcp-lint-rs: $(RS_SOURCES)
	@echo "Building Rust checker..."
	@mkdir -p $(BIN_DIR)
	cd rs && cargo build --release
	cp rs/target/release/mcp-lint-rs $(BIN_DIR)/

rs: $(BIN_DIR)/mcp-lint-rs

rs-test:
	cd rs && cargo test

# --- TypeScript checker ---

TS_SOURCES := $(shell find ts/src -name '*.ts' 2>/dev/null) ts/package.json ts/tsconfig.json

$(BIN_DIR)/mcp-lint-ts: $(TS_SOURCES)
	@echo "Building TypeScript checker..."
	@mkdir -p $(BIN_DIR)
	cd ts && npm run build
	@printf '#!/bin/sh\nnode "$(CURDIR)/ts/dist/main.js" "$$@"\n' > $(BIN_DIR)/mcp-lint-ts
	@chmod +x $(BIN_DIR)/mcp-lint-ts

ts: $(BIN_DIR)/mcp-lint-ts

ts-test:
	cd ts && npm test

# --- Install ---

install: all
	@echo "Installing to ~/bin/..."
	@mkdir -p ~/bin
	cp $(BIN_DIR)/mcp-lint ~/bin/
	cp $(BIN_DIR)/mcp-lint-go ~/bin/
	@test -f $(BIN_DIR)/mcp-lint-rs && cp $(BIN_DIR)/mcp-lint-rs ~/bin/ || true
	@test -f $(BIN_DIR)/mcp-lint-ts && cp $(BIN_DIR)/mcp-lint-ts ~/bin/ || true

# --- Test ---

test: go-test

# --- Clean ---

clean:
	rm -rf $(BIN_DIR)
	cd rs && cargo clean 2>/dev/null || true
	rm -rf ts/dist ts/node_modules 2>/dev/null || true
