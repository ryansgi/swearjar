-- Facts live in Clickhouse, we keep our control-plane in Postgres

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
  id                 UUID,
  event_id           String,
  event_type         String,

  -- 32-byte HIDs as raw bytes
  repo_hid           FixedString(32),
  actor_hid          FixedString(32),
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
      DEFAULT if(length(text_raw) > 0,
                  toInt16(100 * arrayMax(mapValues(detectLanguageMixed(text_raw)))),
                  NULL),

  -- simple reliability flag (tune threshold if desired)
  lang_reliable      UInt8
      DEFAULT ifNull(lang_confidence, 0) >= 60,

  -- script not available from CH NLP; kept for parity/ingest-side fills
  lang_script        Nullable(String),

  -- RU-only sentiment; NULL otherwise
  sentiment_score    Nullable(Float32)
      DEFAULT if(lang_code = 'ru', detectTonality(text_raw), NULL),

  -- ingest metadata / upsert aid (use a stable number per batch; replays can reuse it)
  ingest_batch_id    UInt64 DEFAULT 0,
  ver                UInt64 DEFAULT ingest_batch_id
)
ENGINE = ReplacingMergeTree(ver)
  PARTITION BY toYYYYMM(created_at)
  ORDER BY (repo_hid, created_at, actor_hid, event_id, source, ordinal)
  SETTINGS index_granularity = 8192;

-- Text skipping index (token-based). Note: column is Nullable(String), so index an expression.
CREATE INDEX utt_text_tokenbf ON utterances (coalesce(text_normalized, '')) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

-- If you need aggressive substring search, consider ngrambf_v1 instead (pick one style)
-- CREATE INDEX utt_text_ngrambf ON utterances (coalesce(text_normalized, '')) TYPE ngrambf_v1(3, 2048, 2, 0) GRANULARITY 64;

-- HITS (facts)
CREATE TABLE hits
(
  id                 UUID,
  utterance_id       UUID, -- must match utterances.id
  created_at         DateTime64(3, 'UTC'),

  source             Enum8('commit' = 1, 'issue' = 2, 'pr' = 3, 'comment' = 4),

  repo_hid           FixedString(32),
  actor_hid          FixedString(32),

  lang_code          Nullable(String),
  term               String, -- normalized
  category           Enum8('bot_rage' = 1, 'tooling_rage' = 2, 'self_own' = 3, 'generic' = 4),
  severity           Enum8('mild' = 1, 'strong' = 2, 'slur_masked' = 3),

  span_start         Int32,
  span_end           Int32,
  CONSTRAINT ck_hits_span_valid CHECK span_start >= 0 AND span_end > span_start,

  detector_version   Int32,

  ingest_batch_id    UInt64 DEFAULT 0,
  ver                UInt64 DEFAULT ingest_batch_id
)
ENGINE = ReplacingMergeTree(ver)
  PARTITION BY toYYYYMM(created_at)
  ORDER BY (repo_hid, created_at, actor_hid, term, utterance_id)
  SETTINGS index_granularity = 8192;

-- Fast WHERE term IN (...) / term='...'
CREATE INDEX IF NOT EXISTS hit_term_tokenbf ON hits (term) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 64;

ALTER TABLE utterances MODIFY COLUMN id UUID DEFAULT generateUUIDv7();
ALTER TABLE hits       MODIFY COLUMN id UUID DEFAULT generateUUIDv7();
