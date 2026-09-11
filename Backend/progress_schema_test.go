//go:build integration

package main

import (
	"aigame/server/backend/infra"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestProgressSchemaVersionPolicy(t *testing.T) {
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		t.Skip("run via go run ./cmd/testdb")
	}
	ctx := context.Background()
	p, e := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Pool.Close()
	if e = infra.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); e != nil {
		t.Fatal(e)
	}

	pid := uuid.New()
	if _, e = p.Pool.Exec(ctx, "insert into player(player_id,username,password_hash) values($1,$2,$3)", pid, "schema-"+pid.String(), "x"); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Pool.Exec(ctx, "insert into player_progress(player_id,schema_version) values($1,1)", pid); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); e != nil {
		t.Fatal(e)
	}
	var version int
	if e = p.Pool.QueryRow(ctx, "select schema_version from player_progress where player_id=$1", pid).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if version != infra.ProgressSchemaVersion {
		t.Fatalf("version=%d want=%d", version, infra.ProgressSchemaVersion)
	}

	if _, e = p.Pool.Exec(ctx, "update player_progress set schema_version=99 where player_id=$1", pid); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); !errors.Is(e, infra.ErrProgressSchemaUnsupported) {
		t.Fatalf("expected unsupported migration, got %v", e)
	}
	if _, e = p.GetProgressSnapshot(ctx, pid.String()); !errors.Is(e, infra.ErrProgressSchemaUnsupported) {
		t.Fatalf("expected unsupported snapshot read, got %v", e)
	}
	t.Log("PROGRESS_SCHEMA_POLICY_PASS")
}
