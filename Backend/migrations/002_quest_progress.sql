-- 002: add quest progress storage for authoritative quest objectives.
alter table player_progress add column if not exists quests jsonb not null default '[]'::jsonb;
