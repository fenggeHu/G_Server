-- 002 down: remove quest progress storage.
alter table player_progress drop column if exists quests;
