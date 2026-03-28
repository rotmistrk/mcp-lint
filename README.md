# mcp-lint

Multi-language coding standards enforcement via MCP (Model Context Protocol).

Each language's checker is written in that language using its native AST parser:

| Language   | Written in | AST Library              |
|------------|-----------|--------------------------|
| Go         | Go        | `go/ast` (stdlib)        |
| TypeScript | TypeScript| `typescript` compiler API|
| Rust       | Rust      | `syn`                    |
| C++        | C++       | `libclang`               |
| Java       | Java      | `javaparser`             |

## Architecture

One MCP server (Go) dispatches to per-language checker binaries. Each checker reads a file and outputs JSON violations with a shared contract:

```json
[{"line": 10, "rule": "method-length", "message": "foo is 45 lines (max 40)", "severity": "error"}]
```

## Build

```bash
make              # Build all available checkers + MCP server
make go           # Go checker + MCP server only
make rs           # Rust checker only
make ts           # TypeScript checker only
```

## Install

```bash
make install      # Install to ~/bin/
```

## Configuration

Defaults in `~/.mcp-lint.yaml`, project overrides in `.mcp-lint.yaml`:

```yaml
max_method_length: 40
max_nesting_depth: 3
max_line_width: 120
max_params: 7
max_consecutive_same_type: 2
```

## Kiro Integration

Add to `~/.kiro/settings/mcp.json`:

```json
{
  "mcpServers": {
    "lint": {
      "command": "~/bin/mcp-lint",
      "args": ["--stdio"]
    }
  }
}
```

## License

MIT
