-- You can run these in the default database or a dedicated one (e.g., swearjar).
-- CREATE DATABASE IF NOT EXISTS swearjar;
-- USE swearjar;

-- ---------- Enums (CH Enum8) ----------
-- Matches PG enums; adjust values only by adding to the end to avoid remaps.
-- CH enums are column-local; we declare inline below.

-- ---------- Core: hits (analytics-friendly) ----------
CREATE TABLE IF NOT EXISTS hits_ch
(
  id               UUID,                                     -- from PG uuidv7()
  utterance_id     UUID,                                     -- from PG
  created_at       DateTime64(3) CODEC(DoubleDelta, ZSTD(6)),
  -- enums
  source           Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),
  category         Enum8('bot_rage' = 1, 'tooling_rage' = 2, 'self_own' = 3, 'generic' = 4),
  severity         Enum8('mild' = 1, 'strong' = 2, 'slur_masked' = 3),
  -- dims
  repo_name        LowCardinality(String) CODEC(ZSTD(6)),
  lang_code        LowCardinality(String) CODEC(ZSTD(6)),
  term             LowCardinality(String) CODEC(ZSTD(6)),
  span_start       Int32,
  span_end         Int32,
  detector_version UInt16
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (repo_name, created_at, detector_version, category, severity)
SETTINGS index_granularity = 8192;

-- Optional helper projections (e.g., order by created_at for time slices)
-- CREATE PROJECTION hits_ch_by_time
--   (SELECT * ORDER BY created_at) PRIMARY KEY created_at;

-- ---------- Daily language rollup (auto via MV) ----------
CREATE TABLE IF NOT EXISTS agg_daily_lang_spk_ch
(
  day              Date,
  lang_code        LowCardinality(String),
  detector_version UInt16,
  hits             UInt64,
  events           UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (day, lang_code, detector_version)
SETTINGS index_granularity = 8192;

-- Materialized view to populate the daily rollup from hits_ch.
-- "events" can be derived from distinct utterances per day/lang if desired; for now, pass it in or compute separately.
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_hits_to_daily
TO agg_daily_lang_spk_ch AS
SELECT
  toDate(created_at) AS day,
  lang_code,
  detector_version,
  count()          AS hits,
  uniqExact(utterance_id) AS events
FROM hits_ch
GROUP BY day, lang_code, detector_version;

-- ---------- Governance mirrors (optional; join filters in CH) ----------
-- These are *not* authoritative; sync from PG on a schedule.
CREATE TABLE IF NOT EXISTS repo_opt_outs_ch
(
  repo_name       LowCardinality(String),
  effective_from  DateTime,
  effective_to    Nullable(DateTime),
  reason          String
)
ENGINE = MergeTree
ORDER BY (repo_name, effective_from);

CREATE TABLE IF NOT EXISTS actor_opt_outs_ch
(
  actor_hash      FixedString(32), -- raw 32 bytes rendered as 32-byte string if you prefer; or String for hex(64)
  effective_from  DateTime,
  effective_to    Nullable(DateTime),
  reason          String
)
ENGINE = MergeTree
ORDER BY (actor_hash, effective_from);