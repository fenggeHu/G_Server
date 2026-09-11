-- 001 down: drop all tables and the shared trigger function in dependency order.
drop table if exists progress_operation;
drop table if exists active_player_session;
drop table if exists player_progress;
drop table if exists room_reservation;
drop table if exists room;
drop table if exists ticket;
drop table if exists session;
drop table if exists player;
drop function if exists update_updated_at_column();
