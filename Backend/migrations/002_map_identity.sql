alter table ticket add column if not exists map_id text not null default 'starter_valley';
alter table ticket add column if not exists map_content_version text not null default 'starter_valley-0';
alter table ticket add column if not exists map_authority_version text not null default 'starter_valley-authority-0';
alter table room add column if not exists map_id text not null default 'starter_valley';
alter table room add column if not exists map_content_version text not null default 'starter_valley-0';
alter table room add column if not exists map_authority_version text not null default 'starter_valley-authority-0';
create index if not exists room_map_lookup on room(map_id, map_content_version, map_authority_version, status);
