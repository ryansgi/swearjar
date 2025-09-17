-- Facts in ClickHouse; control-plane in Postgres.

CREATE DATABASE IF NOT EXISTS swearjar;
USE swearjar;

-- Usually I wouldn't recommend using experimental features in prod,
-- but Clickhouse NLP functions are the only game in town for now (and remove a lot of complexity).
-- They are fairly stable, and we can always reprocess data later if needed.
-- See: https://clickhouse.com/docs/en/sql-reference/functions/experimental/nlp/
SET allow_experimental_nlp_functions = 1;

-- UTTERANCES (facts)
CREATE TABLE utterances
(
  -- identity & routing (IDs supplied by app; prefer deterministic for idempotency)
  id UUID,  -- REQUIRED: supplied by app (no DEFAULT)

  -- github's event_id removed: we no longer couple facts to GH/Event IDs
  -- this was done to prevent leaking PII accidentally

  event_type         String,
  repo_hid           FixedString(32), -- 32-byte HIDs as raw bytes
  actor_hid          FixedString(32), -- 32-byte HIDs as raw bytes
  hid_key_version    Int16,
  created_at         DateTime64(3, 'UTC'),
  source             Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),
  source_detail      String,
  ordinal            Int32,
  text_raw           String CODEC(ZSTD(12)),
  text_normalized    Nullable(String) CODEC(ZSTD(12)),

  -- language: auto-detect on insert
  lang_code          Nullable(String)
      DEFAULT if(length(text_raw) > 0, detectLanguage(text_raw), NULL),

  -- best-effort confidence proxy (0..100-ish); guard empty text
  lang_confidence    Nullable(Int16)
      DEFAULT if(length(text_raw) > 0, toInt16(100 * arrayMax(mapValues(detectLanguageMixed(text_raw)))), NULL),

  -- simple reliability flag (tune threshold if desired)
  lang_reliable      UInt8
      DEFAULT ifNull(lang_confidence, 0) >= 60,

  -- RU-only sentiment; NULL otherwise
  sentiment_score    Nullable(Float32)
      DEFAULT if(lang_code = 'ru', detectTonality(text_raw), NULL),

  -- ingest metadata / upsert aid (use a stable number per batch; replays can reuse it)
  ingest_batch_id    UInt64 DEFAULT 0,
  ver                UInt64 DEFAULT ingest_batch_id
)
ENGINE = ReplacingMergeTree(ver)
  PARTITION BY toYYYYMM(created_at)
  -- NOTE: include deterministic id as a tiebreaker for stable ordering/dedup
  ORDER BY (repo_hid, created_at, actor_hid, source, ordinal, id)
  SETTINGS index_granularity = 8192;

-- Text skipping index (token-based). Note: column is Nullable(String), so index an expression.
CREATE INDEX utt_text_tokenbf ON utterances (coalesce(text_normalized, '')) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

-- If you need aggressive substring search, consider ngrambf_v1 instead (pick one style)
-- CREATE INDEX utt_text_ngrambf ON utterances (coalesce(text_normalized, '')) TYPE ngrambf_v1(3, 2048, 2, 0) GRANULARITY 64;

-- HITS (facts)
CREATE TABLE hits
(
  id                 UUID,   -- REQUIRED: supplied by app (no DEFAULT)
  utterance_id       UUID,   -- must match utterances.id
  created_at         DateTime64(3, 'UTC'),
  source             Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),

  repo_hid           FixedString(32),
  actor_hid          FixedString(32),
  lang_code          Nullable(String),

  term               String, -- normalized

  category           Enum8(
                        'bot_rage'     = 1,
                        'tooling_rage' = 2,
                        'self_own'     = 3,
                        'generic'      = 4,
                        'lang_rage'    = 5
                      ),

  severity           Enum8('mild' = 1, 'strong' = 2, 'slur_masked' = 3),

  -- context/category gating diagnostics + targeting (persisted from detector)
  ctx_action         Enum8('none' = 0, 'upgraded' = 1, 'downgraded' = 2) DEFAULT 'none',
  target_type        Enum8('none' = 0, 'bot' = 1, 'tool' = 2, 'lang' = 3, 'framework' = 4) DEFAULT 'none',
  target_id          LowCardinality(String) DEFAULT '',    -- stable alias id from rulepack (e.g., "dependabot", "eslint", "javascript", "react")
  target_name        Nullable(String),                    -- exact surface mention matched (e.g., "@dependabot")
  target_span_start  Nullable(Int32),
  target_span_end    Nullable(Int32),
  target_distance    Nullable(Int32),                     -- bytes from hit center to target start

  span_start         Int32,
  span_end           Int32,
  CONSTRAINT ck_hits_span_valid CHECK span_start >= 0 AND span_end > span_start,

  detector_version   Int32,

  -- detector internals we persist
  detector_source    Enum8('template' = 1, 'lemma' = 2),
  pre_context        String CODEC(ZSTD(12)),
  post_context       String CODEC(ZSTD(12)),
  zones              Array(String) CODEC(ZSTD(12)),

  -- ingest metadata / upsert aid (use a stable number per batch; replays can reuse it)
  ingest_batch_id    UInt64 DEFAULT 0,
  ver                UInt64 DEFAULT ingest_batch_id
)
ENGINE = ReplacingMergeTree(ver)
  PARTITION BY toYYYYMM(created_at)
  ORDER BY (repo_hid, created_at, actor_hid, term, utterance_id, id)
  SETTINGS index_granularity = 8192;

-- Fast WHERE term IN (...) / term='...'
CREATE INDEX IF NOT EXISTS hit_term_tokenbf ON hits (term)
  TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

-- Fast WHERE target_id IN (...) / = '...'
CREATE INDEX IF NOT EXISTS hit_target_tokenbf ON hits (target_id)
  TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

-- ==========================================
-- Nightshift
-- Append-only analytic archives (no time to live)
-- ==========================================

CREATE TABLE term_dict
(
  term_id    UInt64, -- cityHash64(lower(term)) or curated ID
  term       String,
  updated_at DateTime DEFAULT now()
)
ENGINE = MergeTree
  PARTITION BY tuple()
  ORDER BY term_id
  SETTINGS index_granularity = 8192;

CREATE TABLE commit_crimes
(
  -- time & versions
  created_at     DateTime64(3, 'UTC'),
  bucket_hour    DateTime,  -- toStartOfHour(created_at)
  detver         Int32,     -- hits.detector_version

  -- ids
  hit_id         UUID,      -- hits.id
  utterance_id   UUID,      -- hits.utterance_id
  repo_hid       FixedString(32),
  actor_hid      FixedString(32),

  -- source context
  source         Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),
  source_detail  String,

  -- language/context copied from utterances
  lang_code        Nullable(String),
  lang_confidence  Nullable(Int16),
  lang_reliable    UInt8,
  sentiment_score  Nullable(Float32),
  text_len         UInt32, -- length(text_raw)

  -- taxonomy
  term_id        UInt64,
  term           String, -- normalized term
  category       Enum8(
                    'bot_rage'     = 1,
                    'tooling_rage' = 2,
                    'self_own'     = 3,
                    'generic'      = 4,
                    'lang_rage'    = 5
                  ),
  severity       Enum8('mild' = 1, 'strong' = 2, 'slur_masked' = 3),

  -- structured targeting (kept in sync with hits)
  ctx_action         Enum8('none' = 0, 'upgraded' = 1, 'downgraded' = 2) DEFAULT 'none',
  target_type        Enum8('none' = 0, 'bot' = 1, 'tool' = 2, 'lang' = 3, 'framework' = 4) DEFAULT 'none',
  target_id          LowCardinality(String) DEFAULT '',
  target_name        Nullable(String),
  target_span_start  Nullable(Int32),
  target_span_end    Nullable(Int32),
  target_distance    Nullable(Int32),

  -- span info (for samples/drilldowns)
  span_start     Int32,
  span_end       Int32,

  -- arry-through detector context
  detector_source Enum8('template' = 1, 'lemma' = 2),
  pre_context     String CODEC(ZSTD(12)),
  post_context    String CODEC(ZSTD(12)),
  zones           Array(String) CODEC(ZSTD(12))
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (bucket_hour, repo_hid, actor_hid, term_id, detver, utterance_id, hit_id)
SETTINGS index_granularity = 8192;

CREATE INDEX IF NOT EXISTS cc_term_tokenbf ON commit_crimes (term)
  TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

CREATE INDEX IF NOT EXISTS cc_target_tokenbf ON commit_crimes (target_id)
  TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;


CREATE VIEW v_cc_timeseries_daily AS
SELECT toDate(created_at) AS day, detver, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY day, detver
ORDER BY day ASC, detver ASC;

CREATE VIEW v_cc_timeseries_hourly AS
SELECT toStartOfHour(created_at) AS hour, detver, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY hour, detver
ORDER BY hour ASC, detver ASC;

CREATE VIEW v_cc_hits_by_detver_daily AS
SELECT toDate(created_at) AS day, detver, count() AS hits
FROM commit_crimes
GROUP BY day, detver
ORDER BY day ASC, detver ASC;

CREATE VIEW v_cc_heatmap_weekly AS
SELECT toDayOfWeek(created_at) - 1 AS dow, toHour(created_at) AS hod, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY dow, hod
ORDER BY dow ASC, hod ASC;

CREATE VIEW v_cc_lang_breakdown AS
SELECT coalesce(lang_code, '') AS lang_code, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY lang_code
ORDER BY hits DESC;

-- Category x severity
CREATE VIEW v_cc_category_severity AS
SELECT cast(category AS Nullable(String)) AS category, cast(severity AS Nullable(String)) AS severity, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY category, severity
ORDER BY hits DESC;

-- Top terms per hour (ranked in-view via window function; keep all detvers/langs)
CREATE VIEW v_cc_top_terms_hour AS
WITH base AS (
  SELECT
    toStartOfHour(created_at) AS bucket_hour,
    detver,
    lang_code AS nl_lang,
    term_id,
    anyHeavy(term) AS term,
    count() AS hits
  FROM commit_crimes
  GROUP BY bucket_hour, detver, nl_lang, term_id
),
ranked AS (
  SELECT
    *,
    row_number() OVER (PARTITION BY bucket_hour, detver ORDER BY hits DESC, term_id ASC) AS rn
  FROM base
)
SELECT * FROM ranked WHERE rn <= 20 ORDER BY bucket_hour ASC, detver ASC, hits DESC;

CREATE VIEW v_cc_repo_day AS
SELECT
  toDate(created_at) AS day,
  repo_hid,
  detver,
  cast(category AS Nullable(String)) AS category,
  cast(severity AS Nullable(String)) AS severity,
  count() AS hits,
  uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY day, repo_hid, detver, category, severity
ORDER BY day ASC, repo_hid ASC, detver ASC, hits DESC;

-- Samples: latest N per (term_id, repo_hid) keeps drilldowns simple
-- (filter with WHERE before selecting from this view to keep it small)
CREATE VIEW v_cc_samples_latest AS
SELECT * FROM
(
  SELECT
    c.*,
    row_number() OVER (
      PARTITION BY term_id, repo_hid
      ORDER BY created_at DESC, hit_id ASC
    ) AS rn
  FROM commit_crimes AS c
)
WHERE rn <= 50;

CREATE VIEW v_cc_actor_timeseries_daily AS
SELECT toDate(created_at) AS day, actor_hid, detver, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY day, actor_hid, detver
ORDER BY day ASC, actor_hid ASC, detver ASC;

CREATE VIEW v_cc_actor_timeseries_hourly AS
SELECT toStartOfHour(created_at) AS hour, actor_hid, detver, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY hour, actor_hid, detver
ORDER BY hour ASC, actor_hid ASC, detver ASC;

CREATE VIEW v_cc_actor_day AS
SELECT
  toDate(created_at) AS day,
  actor_hid,
  detver,
  cast(category AS Nullable(String)) AS category,
  cast(severity AS Nullable(String)) AS severity,
  count() AS hits,
  uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY day, actor_hid, detver, category, severity
ORDER BY day ASC, actor_hid ASC, detver ASC, hits DESC;

CREATE VIEW v_cc_actor_lang_breakdown AS
SELECT actor_hid, coalesce(lang_code, '') AS lang_code, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY actor_hid, lang_code
ORDER BY actor_hid ASC, hits DESC;

CREATE VIEW v_cc_actor_category_severity AS
SELECT actor_hid, cast(category AS Nullable(String)) AS category, cast(severity AS Nullable(String)) AS severity, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY actor_hid, category, severity
ORDER BY actor_hid ASC, hits DESC;

CREATE VIEW v_cc_actor_top_terms_daily AS
WITH base AS (
  SELECT
    toDate(created_at) AS day,
    actor_hid,
    detver,
    lang_code AS nl_lang,
    term_id,
    anyHeavy(term) AS term,
    count() AS hits
  FROM commit_crimes
  GROUP BY day, actor_hid, detver, nl_lang, term_id
),
ranked AS (
  SELECT
    *,
    row_number() OVER (PARTITION BY day, actor_hid, detver ORDER BY hits DESC, term_id ASC) AS rn
  FROM base
)
SELECT *
FROM ranked
WHERE rn <= 20
ORDER BY day ASC, actor_hid ASC, detver ASC, hits DESC;

-- Actor leaderboard (hits & unique-utterance counts) over any filtered window
CREATE VIEW v_cc_actor_leaderboard AS
SELECT actor_hid, count() AS hits, uniqCombined(12)(utterance_id) AS utterances, min(created_at) AS first_seen_at, max(created_at) AS last_seen_at
FROM commit_crimes
GROUP BY actor_hid
ORDER BY hits DESC, utterances DESC;

-- Actor x repo cross-tab daily (helps see where an actor's events happen)
CREATE VIEW v_cc_actor_repo_day AS
SELECT toDate(created_at) AS day, actor_hid, repo_hid, detver, count() AS hits, uniqCombined(12)(utterance_id) AS utterances
FROM commit_crimes
GROUP BY day, actor_hid, repo_hid, detver
ORDER BY day ASC, actor_hid ASC, hits DESC;

-- Hourly Aggregates (utterances)
CREATE TABLE utt_hour_agg
(
  -- Dimensions
  bucket_hour     DateTime,  -- UTC hour
  repo_hid        FixedString(32),
  actor_hid       FixedString(32),
  source          Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),
  lang_code       LowCardinality(Nullable(String)),
  lang_reliable   UInt8,

  -- Core metrics
  u_state         AggregateFunction(uniq, UUID),                           -- uniqState(id)
  cnt_state       AggregateFunction(count),                                -- countState()

  -- Text length
  text_sum_state  AggregateFunction(sum, UInt64),                          -- sumState(length(text_raw))
  text_q_state    AggregateFunction(quantilesTDigest(0.5, 0.9, 0.99), Float64),

  -- Sentiment (RU-only; ignore NULLs)
  sent_cnt_state  AggregateFunction(countIf, UInt8),
  sent_avg_state  AggregateFunction(avgIf,   Float64, UInt8),
  sent_q_state    AggregateFunction(quantilesTDigestIf(0.5, 0.9, 0.99), Float64, UInt8),

  -- Time sanity
  min_at_state    AggregateFunction(min, DateTime64(3)),
  max_at_state    AggregateFunction(max, DateTime64(3))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(bucket_hour)
ORDER BY (bucket_hour, repo_hid, actor_hid, source, ifNull(lang_code, ''), lang_reliable)
SETTINGS index_granularity = 8192;
