create or replace function update_updated_at_column() returns trigger as $$ begin new.updated_at = now(); return new; end; $$ language plpgsql;

create table if not exists player(player_id uuid primary key, username text unique not null, password_hash text not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), status smallint not null default 1);
create trigger trg_player_updated_at before update on player for each row execute function update_updated_at_column();

create table if not exists session(id bigserial primary key, player_id uuid references player, token_hash text unique not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), expires_at timestamptz not null, revoked_at timestamptz, status smallint not null default 1);
create trigger trg_session_updated_at before update on session for each row execute function update_updated_at_column();

create table if not exists ticket(token_hash text primary key, player_id uuid references player, session_hash text not null, room_id text not null, preset_id text not null, resolved_config_hash text not null, protocol_version int not null, gameplay_content_hash text not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), expires_at timestamptz not null, redeemed_at timestamptz, status smallint not null default 1);
create trigger trg_ticket_updated_at before update on ticket for each row execute function update_updated_at_column();

create index if not exists session_player on session(player_id,created_at desc);

create table if not exists room(room_id text primary key, host text not null, port int not null, protocol_version int not null, gameplay_content_hash text not null, resolved_config_hash text not null, capacity int not null check(capacity between 1 and 8), generation int not null, status text not null, used_players int not null default 0, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), last_heartbeat timestamptz not null);
create trigger trg_room_updated_at before update on room for each row execute function update_updated_at_column();

create table if not exists room_reservation(room_id text references room(room_id) on delete cascade, player_id uuid references player(player_id) on delete cascade, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), released_at timestamptz, status smallint not null default 1, primary key(room_id,player_id));
create trigger trg_room_reservation_updated_at before update on room_reservation for each row execute function update_updated_at_column();

create unique index if not exists active_player_reservation on room_reservation(player_id) where released_at is null;

create table if not exists player_progress(player_id uuid primary key references player(player_id) on delete cascade, schema_version int not null default 1, revision bigint not null default 0, inventory jsonb not null default '{}'::jsonb, equipment jsonb not null default '{}'::jsonb, unlocks jsonb not null default '[]'::jsonb, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), status smallint not null default 1);
create trigger trg_player_progress_updated_at before update on player_progress for each row execute function update_updated_at_column();

create table if not exists active_player_session(player_id uuid primary key references player(player_id) on delete cascade, fencing_token bigint not null, room_id text not null, lease_expires_at timestamptz not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), status smallint not null default 1);
create trigger trg_active_player_session_updated_at before update on active_player_session for each row execute function update_updated_at_column();

create table if not exists progress_operation(player_id uuid not null references player(player_id) on delete cascade, operation_id text not null, payload_hash text not null, status text not null, result jsonb not null default '{}'::jsonb, revision bigint not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), primary key(player_id, operation_id));
create trigger trg_progress_operation_updated_at before update on progress_operation for each row execute function update_updated_at_column();