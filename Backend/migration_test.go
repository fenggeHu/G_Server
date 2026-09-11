//go:build integration

package main

import (
	"aigame/server/backend/infra"
	"context"
	"os"
	"testing"
)

func TestMigrationRollback(t *testing.T) {
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
	if !migrationColumnExists(t, ctx, p, "player_progress", "quests") {
		t.Fatal("quests column missing after migrate")
	}

	rolled, e := infra.Rollback(ctx, p, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(rolled) != 1 || rolled[0] != "002_quest_progress.sql" {
		t.Fatalf("unexpected rollback set: %v", rolled)
	}
	if migrationColumnExists(t, ctx, p, "player_progress", "quests") {
		t.Fatal("quests column still present after rollback")
	}

	if e = infra.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	if !migrationColumnExists(t, ctx, p, "player_progress", "quests") {
		t.Fatal("quests column not restored after re-migrate")
	}

	rolled, e = infra.Rollback(ctx, p, 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(rolled) != 2 {
		t.Fatalf("expected both migrations rolled back, got %v", rolled)
	}
	if migrationTableExists(t, ctx, p, "player_progress") {
		t.Fatal("player_progress still present after full rollback")
	}

	if e = infra.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	if !migrationTableExists(t, ctx, p, "player_progress") {
		t.Fatal("player_progress not restored after final migrate")
	}
	t.Log("MIGRATION_ROLLBACK_PASS")
}

func migrationColumnExists(t *testing.T, ctx context.Context, p *infra.PG, table, column string) bool {
	t.Helper()
	var n int
	if e := p.Pool.QueryRow(ctx, "select count(*) from information_schema.columns where table_schema=current_schema() and table_name=$1 and column_name=$2", table, column).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n > 0
}

func migrationTableExists(t *testing.T, ctx context.Context, p *infra.PG, table string) bool {
	t.Helper()
	var n int
	if e := p.Pool.QueryRow(ctx, "select count(*) from information_schema.tables where table_schema=current_schema() and table_name=$1", table).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n > 0
}
