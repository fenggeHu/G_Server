# G0 Go Backend

Cross-project authentication smoke test (temporary PostgreSQL; Python standard library only for orchestration, no Python backend):

```sh
go run ./cmd/testdb python3 ../../3D_App/Tests/g0_integration.py
```

This directory implements the G0 HTTP API, PostgreSQL migrations, Argon2id account seeding, session/ticket security rules, and isolated integration testing.

Required environment: `DATABASE_URL`, `ROOM_SERVICE_TOKEN` (minimum 32 characters). Run `go run . migrate`, then `G0_TEST_PASSWORD='...' go run . seed USERNAME`, then `ALLOW_LOOPBACK_HTTP=1 go run .` for loopback development. Production requires `TLS_CERT_FILE` and `TLS_KEY_FILE`.

The pure-Go disposable PostgreSQL runner creates a random `postgres:17.6` container, random password, Docker-assigned loopback port, and random schema. It removes only its own container in a deferred cleanup. With no child command it runs the race suite:

```sh
go run ./cmd/testdb
go run ./cmd/testdb go test -race ./...
```

Any child command inherits `DATABASE_URL`, `TEST_DB_SCHEMA`, `ROOM_SERVICE_TOKEN`, and `TEST_DB_INTEGRATION=1`; credentials are not printed. The API integration test creates and removes only its own schema. Coverage from the default unit-only run is intentionally reported honestly; use the runner with `go test -coverprofile=coverage.out ./...` to include real PostgreSQL API coverage.
# G1 Local Integration

Run `go run ./cmd/testdb go run ./cmd/g1integration` from this directory.
The outer Go runner owns temporary Docker PostgreSQL; the inner runner builds and starts Backend, one Godot GameServer and two independent Godot clients. It verifies late join across the lease boundary, both clients' self/remote joins, successful exits, server leave events, registration fields and separate redeemed tickets. Run `go run ./cmd/testdb go test -race -v ./...` and `go vet ./...` for database and control-plane regressions.
