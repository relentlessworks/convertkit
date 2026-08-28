# convertkit

Agentic-first data format conversion service. Convert between JSON, YAML, TOML, CSV, XML, Properties, JSONL, and Env formats. Plain text API, agent-driven, single Go binary.

## Quick Start

```bash
make build
./convertkit
```

The server starts on `:7700` by default.

## Auth Flow

```bash
# 1. Request OTP
curl -X POST -d 'email=agent@example.com' http://localhost:7700/auth/request
# → status=ok message=OTP sent to agent@example.com (check stderr in dev mode) workspace=ws_xxxx

# 2. Verify OTP (check stderr for the code)
curl -X POST -d 'email=agent@example.com&code=123456' http://localhost:7700/auth/verify
# → token=abc123... workspace=ws_xxxx

# 3. Convert data
curl -X POST -H 'Authorization: Bearer abc123...' \
  -d '{"name":"test","value":42}' \
  'http://localhost:7700/convert?from=json&to=yaml'
# → name: test
#   value: 42
```

## Supported Formats

| Format | Description |
|--------|-------------|
| `json` | JavaScript Object Notation |
| `yaml` | YAML Ain't Markup Language |
| `toml` | Tom's Obvious Minimal Language |
| `csv` | Comma-Separated Values |
| `xml` | eXtensible Markup Language |
| `properties` | Java Properties (key=value) |
| `jsonl` | JSON Lines (one object per line) |
| `env` | Environment variables (KEY=value) |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/help` | No | One-page operating manual |
| GET | `/.well-known/agent.md` | No | Same as /help |
| GET | `/formats` | No | List supported formats |
| POST | `/auth/request` | No | Request OTP via email |
| POST | `/auth/verify` | No | Verify OTP, get bearer token |
| POST | `/convert?from=X&to=Y` | Yes | Convert data between formats |
| GET | `/history?limit=N` | Yes | List recent conversions |
| GET | `/history/{handle}` | Yes | Get a specific conversion |
| GET | `/workspace` | Yes | Show workspace info |
| GET | `/audit?limit=N` | Yes | Show audit log |
| POST | `/mcp` | No* | MCP JSON-RPC 2.0 endpoint |

*The `convert` and `list_formats` MCP tools don't require auth. `get_history` requires a token in the arguments.

## Response Format

Plain text by default. Add `Accept: application/json` or `?format=json` for JSON.

Errors include a hint:
```
error: unsupported source format: ini | hint: use one of: json, yaml, toml, csv, xml, properties, jsonl, env
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `-addr` | `CONVERTKIT_ADDR` | `:7700` | Listen address |
| `-db` | `CONVERTKIT_DB` | `convertkit.json` | Database file path |
| `-secret` | `CONVERTKIT_SECRET` | (auto) | Token signing secret |

## Build

```bash
make build    # Build the binary
make test     # Run tests
make vet      # Run go vet
```

## License

MIT
