# Roadmap

## Deep Code Analysis: Gaps and Issues

The current implementation is a single-file TCP server backed by an in-memory SQLite
database. The following gaps were identified:

1. Extension paths are hard-coded and unavailable in non-Docker environments.
2. Transaction logging uses `TIME('now')` as a primary key, causing collisions.
3. Only `SELECT` and `PRAGMA` statements are routed to `runQuery`.
4. Result rendering forces all values through `sql.NullString`.
5. SQLite `:memory:` databases are not shared across connections.
6. `acceptLoop` cannot be interrupted without closing the listener.
7. No graceful shutdown of active connections.
8. No query timeouts or context cancellation.
9. No input length limits (potential memory exhaustion).
10. No metrics or observability hooks.
11. No authentication or authorization.
12. No TLS support.
13. No prepared statement caching.
14. No connection pooling/back-pressure controls.
15. No pagination for large result sets.
16. No audit trail beyond best-effort logging.
17. Fuzz coverage was missing for SQL command paths.
18. Unit tests were deadlock-prone and incomplete.
19. No performance or memory profiling targets.
20. No pre-commit verification hook.

## 20 Improvement / Performance / Accuracy Items

1. Make sqlite-vss extension loading optional and environment-driven.
2. Replace timestamp primary keys with auto-increment IDs.
3. Route `WITH` and `EXPLAIN` statements through `runQuery`.
4. Render typed values (`int64`, `float64`, `[]byte`) directly.
5. Use shared-cache in-memory databases with `MaxOpenConns(1)`.
6. Add listener close handling for clean shutdown.
7. Add graceful connection draining on shutdown.
8. Add per-query context with timeout support.
9. Enforce maximum input line length.
10. Add structured metrics for query latency and errors.
11. Add optional authentication for TCP clients.
12. Add optional TLS listener support.
13. Cache prepared statements for repeated queries.
14. Add bounded connection acceptance/back-pressure.
15. Add result-set pagination or row limits.
16. Persist audit logs to durable storage.
17. Expand fuzz targets to all command handlers.
18. Expand unit tests to cover initDB and acceptLoop edge cases.
19. Add benchmarks for hot paths (readUntil, handleDotCommand).
20. Add pre-commit verification via `scripts/verify.sh`.

## Quality Gates

- `make ci` runs formatting, lint, vet, build, coverage, fuzz, accuracy, oracle, security, and helm checks.
- `make accuracy-gate` validates SQL result correctness.
- `make oracle-gate` validates parity against expected results.
- `make fuzz-coverage` enforces fuzz targets in every package.
- `make security` runs gosec and govulncheck.
- `make perf-check`, `make mem-check`, and profiling targets exercise performance and memory.