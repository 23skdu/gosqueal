# gosqueal

![CI](https://github.com/23skdu/gosqueal/actions/workflows/ci.yml/badge.svg?branch=fix/standardize-layout)

A simple TCP server that logs and executes received SQL queries against an in-memory SQLite database. Useful for testing or as a honeypot.

## Architecture

- **Language**: Go
- **Database**: SQLite (modernc.org/sqlite - pure Go)
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
