create table if not exists players(player_id uuid primary key, username text unique not null, password_hash text not null);
create table if not exists sessions(id bigserial primary key, player_id uuid references players, token_hash text unique not null, created_at timestamptz default now(), expires_at timestamptz not null, revoked_at timestamptz);
create table if not exists tickets(token_hash text primary key, player_id uuid references players, session_hash text not null, room_id text not null, preset_id text not null, resolved_config_hash text not null, protocol_version int not null, gameplay_content_hash text not null, expires_at timestamptz not null, redeemed_at timestamptz);
create index if not exists sessions_player on sessions(player_id,created_at desc);
