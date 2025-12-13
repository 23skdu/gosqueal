[![CI](https://github.com/23skdu/gosqueal/actions/workflows/ci.yml/badge.svg)](https://github.com/23skdu/gosqueal/actions/workflows/ci.yml)
[![Release](https://github.com/23skdu/gosqueal/actions/workflows/release.yml/badge.svg)](https://github.com/23skdu/gosqueal/actions/workflows/release.yml)
[![Markdown Lint](https://github.com/23skdu/gosqueal/actions/workflows/markdown-lint.yml/badge.svg)](https://github.com/23skdu/gosqueal/actions/workflows/markdown-lint.yml)
[![Helm Validation](https://github.com/23skdu/gosqueal/actions/workflows/helm-validation.yml/badge.svg)](https://github.com/23skdu/gosqueal/actions/workflows/helm-validation.yml)

# gosqueal

A simple TCP server that logs and executes received SQL queries against an
in-memory SQLite database for Agents

## Architecture

- **Language**: Go
- **Database**: SQLite (github.com/mattn/go-sqlite3 - CGO enabled)
- **Protocol**: TCP

## Development

### Build

```bash
go build ./cmd/gosqueal
```

### Run

```bash
./gosqueal -port 1118
```

## Docker

```bash
docker build -t gosqueal .
```
