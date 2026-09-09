package infra

import (
	"aigame/server/backend/domain"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
)

type PG struct {
	Pool   *pgxpool.Pool
	Schema string
}

func Open(ctx context.Context, url, schema string) (*PG, error) {
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	if schema != "" {
		for _, r := range schema {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
				return nil, fmt.Errorf("invalid database schema")
			}
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	return &PG{p, schema}, nil
}
func (p *PG) q(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.Pool.QueryRow(ctx, sql, args...)
}
func pgxIdent(s string) string {
	if s == "" {
		return "public"
	}
	return pgx.Identifier{s}.Sanitize()
}
func (p *PG) FindPlayer(c context.Context, u string) (domain.Player, error) {
	var x domain.Player
	e := p.q(c, "select player_id,password_hash from players where username=$1", u).Scan(&x.ID, &x.PasswordHash)
	return x, e
}
func (p *PG) CreateSession(c context.Context, id, h string) error {
	_, e := p.Pool.Exec(c, "insert into sessions(player_id,token_hash,expires_at) values($1,$2,now()+interval '15 minutes')", id, h)
	return e
}
func (p *PG) FindSession(c context.Context, h string) (domain.Session, error) {
	var x domain.Session
	e := p.q(c, "select player_id,token_hash from sessions where token_hash=$1 and revoked_at is null and expires_at>now()", h).Scan(&x.PlayerID, &x.TokenHash)
	return x, e
}
func (p *PG) RevokeSession(c context.Context, h string) error {
	_, e := p.Pool.Exec(c, "update sessions set revoked_at=now() where token_hash=$1", h)
	return e
}
func (p *PG) IssueTicket(c context.Context, x domain.Ticket, ticketHash, sessionHash string) error {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return e
	}
	defer tx.Rollback(c)
	var id string
	if e = tx.QueryRow(c, "select player_id from sessions where token_hash=$1 and player_id=$2 and revoked_at is null and expires_at>now() for update", sessionHash, x.PlayerID).Scan(&id); e != nil {
		return e
	}
	_, e = tx.Exec(c, "insert into tickets(token_hash,player_id,session_hash,expires_at,room_id,preset_id,resolved_config_hash,protocol_version,gameplay_content_hash) values($1,$2,$3,now()+interval '60 seconds',$4,$5,$6,$7,$8)", ticketHash, id, sessionHash, x.RoomID, x.PresetID, x.ConfigHash, x.Protocol, x.Content)
	if e != nil {
		return e
	}
	return tx.Commit(c)
}
func (p *PG) RedeemTicket(c context.Context, x domain.Ticket) (domain.Ticket, error) {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return x, e
	}
	defer tx.Rollback(c)
	var out domain.Ticket
	var session string
	if e = tx.QueryRow(c, "select s.token_hash from sessions s join tickets t on t.session_hash=s.token_hash where t.token_hash=$1 for update of s", domainHash(x.Token)).Scan(&session); e != nil {
		return x, e
	}
	e = tx.QueryRow(c, "update tickets set redeemed_at=now() where token_hash=$1 and redeemed_at is null and expires_at>now() and room_id=$2 and protocol_version=$3 and gameplay_content_hash=$4 and resolved_config_hash=$5 and preset_id='exploration' and exists(select 1 from sessions s where s.token_hash=tickets.session_hash and s.player_id=tickets.player_id and s.revoked_at is null and s.expires_at>now()) returning player_id,room_id,preset_id,resolved_config_hash", domainHash(x.Token), x.RoomID, x.Protocol, x.Content, x.ConfigHash).Scan(&out.PlayerID, &out.RoomID, &out.PresetID, &out.ConfigHash)
	if e != nil {
		return x, e
	}
	out.Token = x.Token
	if e = tx.Commit(c); e != nil {
		return x, e
	}
	return out, nil
}
func domainHash(s string) string           { b := sha256.Sum256([]byte(s)); return fmt.Sprintf("%x", b) }
func (p *PG) Ping(c context.Context) error { return p.Pool.Ping(c) }
func Migrate(ctx context.Context, p *PG) error {
	path := os.Getenv("MIGRATIONS_DIR")
	if path == "" {
		path = "migrations"
	}
	b, e := os.ReadFile(filepath.Join(path, "001_init.sql"))
	if e != nil {
		return e
	}
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if p.Schema != "" {
		if _, e = tx.Exec(ctx, "create schema if not exists "+pgxIdent(p.Schema)); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "select pg_advisory_xact_lock(884422)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "create table if not exists schema_migrations(name text primary key,checksum text not null)"); e != nil {
		return e
	}
	var old string
	e = tx.QueryRow(ctx, "select checksum from schema_migrations where name='001_init.sql'").Scan(&old)
	if e == nil {
		if old != domainHash(string(b)) {
			return fmt.Errorf("migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if e != pgx.ErrNoRows {
		return e
	}
	if _, e = tx.Exec(ctx, string(b)); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "insert into schema_migrations values('001_init.sql',$1)", domainHash(string(b))); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
