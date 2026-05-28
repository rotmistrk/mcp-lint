.PHONY: all go rs ts cpp java install clean

BIN_DIR := bin

all: go rs ts cpp java

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

$(BIN_DIR)/mcp-lint-cli: $(GO_SOURCES)
	@echo "Building CLI tool..."
	@mkdir -p $(BIN_DIR)
	cd go && go build -ldflags="-w -s" -o ../$(BIN_DIR)/mcp-lint-cli ./cmd/mcp-lint-cli

go: $(BIN_DIR)/mcp-lint $(BIN_DIR)/mcp-lint-go $(BIN_DIR)/mcp-lint-cli

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
	cp ts/dist/mcp-lint-ts.cjs $(BIN_DIR)/mcp-lint-ts
	chmod +x $(BIN_DIR)/mcp-lint-ts

ts: $(BIN_DIR)/mcp-lint-ts

ts-test:
	cd ts && npm test

# --- C++ checker ---

CPP_SOURCES := $(shell find cpp/src -name '*.cpp' -o -name '*.h' 2>/dev/null) cpp/CMakeLists.txt

$(BIN_DIR)/mcp-lint-cpp: $(CPP_SOURCES)
	@echo "Building C++ checker..."
	@mkdir -p cpp/build $(BIN_DIR)
	cd cpp/build && cmake -DCMAKE_BUILD_TYPE=Release .. && cmake --build . --parallel
	cp cpp/build/mcp-lint-cpp $(BIN_DIR)/

cpp: $(BIN_DIR)/mcp-lint-cpp

# --- Java checker ---

$(BIN_DIR)/mcp-lint-java: $(shell find java/src -name '*.java' 2>/dev/null) java/build.gradle
	@echo "Building Java checker..."
	@mkdir -p $(BIN_DIR)
	cd java && ./gradlew jar -q
	@printf '#!/bin/sh\njava -jar "$(CURDIR)/java/build/libs/mcp-lint-java.jar" "$$@"\n' > $(BIN_DIR)/mcp-lint-java
	@chmod +x $(BIN_DIR)/mcp-lint-java

java: $(BIN_DIR)/mcp-lint-java

# --- Install ---

install: all
	@echo "Installing to ~/bin/..."
	@mkdir -p ~/bin
	install -m 755 $(BIN_DIR)/mcp-lint ~/bin/
	install -m 755 $(BIN_DIR)/mcp-lint-go ~/bin/
	install -m 755 $(BIN_DIR)/mcp-lint-cli ~/bin/
	@test -f $(BIN_DIR)/mcp-lint-rs && install -m 755 $(BIN_DIR)/mcp-lint-rs ~/bin/ || true
	@test -f $(BIN_DIR)/mcp-lint-ts && install -m 755 $(BIN_DIR)/mcp-lint-ts ~/bin/ || true
	@test -f $(BIN_DIR)/mcp-lint-cpp && install -m 755 $(BIN_DIR)/mcp-lint-cpp ~/bin/ || true
	@test -f $(BIN_DIR)/mcp-lint-java && install -m 755 $(BIN_DIR)/mcp-lint-java ~/bin/ || true

# --- Test ---

test: go-test

# --- Clean ---

clean:
	rm -rf $(BIN_DIR)
	cd rs && cargo clean 2>/dev/null || true
	rm -rf ts/dist ts/node_modules 2>/dev/null || true
