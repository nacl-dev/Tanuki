-- 001_initial.down.sql
-- Drop all tables in reverse dependency order

DROP TABLE IF EXISTS download_schedules;
DROP TABLE IF EXISTS download_jobs;
DROP TABLE IF EXISTS media_performers;
DROP TABLE IF EXISTS performers;
DROP TABLE IF EXISTS media_collections;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS media_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS media;
DROP TABLE IF EXISTS schema_migrations;
