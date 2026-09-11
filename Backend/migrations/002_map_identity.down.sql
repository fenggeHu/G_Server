drop index if exists room_map_lookup;
alter table ticket drop column if exists map_id;
alter table ticket drop column if exists map_content_version;
alter table ticket drop column if exists map_authority_version;
alter table room drop column if exists map_id;
alter table room drop column if exists map_content_version;
alter table room drop column if exists map_authority_version;
