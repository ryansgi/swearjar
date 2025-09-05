-- PG INIT SCRIPT FOR SWEARJAR
-- Run once on DB init
-- @TODO: Migrations system

-- This is heavily a work in progress. Expect breaking changes

-- Extensions
CREATE EXTENSION "uuid-ossp";
CREATE EXTENSION pg_trgm;

-- Enums
CREATE TYPE source_enum AS ENUM ('commit','issue','pr','comment');
CREATE TYPE hit_category_enum AS ENUM ('bot_rage','tooling_rage','self_own','generic');
CREATE TYPE hit_severity_enum AS ENUM ('mild','strong','slur_masked');
CREATE TYPE stop_kind_enum AS ENUM ('exact','substring','regex');
CREATE TYPE principal_enum AS ENUM ('repo','actor');
CREATE TYPE consent_action_enum AS ENUM ('opt_in','opt_out');
CREATE TYPE consent_state_enum AS ENUM ('pending','active','revoked','expired');
CREATE TYPE consent_scope_enum AS ENUM ('demask_repo','demask_self');
CREATE TYPE evidence_kind_enum AS ENUM ('repo_file','actor_gist');

-- Domains
CREATE DOMAIN sha256_bytes AS bytea CHECK (octet_length(VALUE) = 32);
CREATE DOMAIN hid_bytes   AS bytea CHECK (octet_length(VALUE) = 32);

-- Rulepack meta (versioning)
CREATE TABLE rulepacks (
  version           int PRIMARY KEY,
  created_at        timestamptz NOT NULL DEFAULT now(),
  description       text,
  checksum_sha256   bytea
);

-- Facts: utterances (HIDs + numeric ids first-class)
CREATE TABLE utterances (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  event_id         text        NOT NULL,
  event_type       text        NOT NULL,
  repo_name        text        NOT NULL,
  repo_id          bigint      NOT NULL,
  repo_hid         hid_bytes   NOT NULL,
  actor_login      text        NOT NULL,
  actor_id         bigint      NOT NULL,
  actor_hid        hid_bytes   NOT NULL,
  hid_key_version  smallint    NOT NULL,
  created_at       timestamptz NOT NULL,
  source           source_enum NOT NULL,
  source_detail    text        NOT NULL DEFAULT '',
  ordinal          int         NOT NULL,
  text_raw         text        NOT NULL,
  text_normalized  text,
  lang_code        text,
  script           text
);

CREATE UNIQUE INDEX ux_utterances_event_source_ord ON utterances(event_id, source, ordinal);
CREATE INDEX ix_utterances_repo_time ON utterances(repo_name, created_at);
CREATE INDEX ix_utterances_type_time ON utterances(event_type, created_at);
CREATE INDEX ix_utterances_text_norm_trgm ON utterances USING gin (text_normalized gin_trgm_ops);
CREATE INDEX ix_utterances_repo_hid_time ON utterances(repo_hid, created_at);
CREATE INDEX ix_utterances_actor_hid_time ON utterances(actor_hid, created_at);
CREATE INDEX ix_utterances_created_at ON utterances(created_at);
CREATE INDEX ix_utterances_created_id ON utterances (created_at, id);

-- Derived: hits (store HIDs for hot filters)
CREATE TABLE hits (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  utterance_id     uuid NOT NULL REFERENCES utterances(id) ON DELETE CASCADE,
  created_at       timestamptz NOT NULL DEFAULT now(),
  source           source_enum NOT NULL,
  repo_name        text NOT NULL,
  repo_hid         hid_bytes,
  actor_hid        hid_bytes,
  lang_code        text,
  term             text NOT NULL,
  category         hit_category_enum NOT NULL,
  severity         hit_severity_enum NOT NULL,
  span_start       int NOT NULL,
  span_end         int NOT NULL,
  detector_version int NOT NULL REFERENCES rulepacks(version)
);

CREATE UNIQUE INDEX uniq_hits_semantic ON hits (utterance_id, term, span_start, span_end, detector_version);
CREATE INDEX idx_hits_repo_created ON hits (repo_name, created_at);
CREATE INDEX idx_hits_repo_hid_created ON hits (repo_hid, created_at);
CREATE INDEX idx_hits_actor_hid_created ON hits (actor_hid, created_at);
CREATE INDEX idx_hits_cat_sev ON hits (category, severity);
CREATE INDEX idx_hits_lang ON hits (lang_code);
CREATE INDEX idx_hits_term_created ON hits (term, created_at);
CREATE INDEX idx_hits_created_at ON hits(created_at);
CREATE INDEX idx_hits_detver_created ON hits (detector_version, created_at);

ALTER TABLE hits ADD CONSTRAINT ck_hits_span_valid CHECK (span_start >= 0 AND span_end > span_start);

-- Aggregates (PG remains the SoT)
CREATE TABLE agg_daily_lang_spk (
  day              date NOT NULL,
  lang_code        text,
  hits             bigint NOT NULL,
  events           bigint NOT NULL,
  detector_version int NOT NULL REFERENCES rulepacks(version),
  PRIMARY KEY (day, lang_code, detector_version)
);

-- Governance: stoplists
CREATE TABLE stop_terms (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  term_norm    text UNIQUE NOT NULL,
  kind         stop_kind_enum NOT NULL,
  active       boolean NOT NULL DEFAULT true,
  notes        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Consent challenges and receipts
CREATE TABLE consent_challenges (
  challenge_hash     text PRIMARY KEY,
  principal          principal_enum NOT NULL,
  resource           text NOT NULL,
  action             consent_action_enum NOT NULL,
  scope              consent_scope_enum[],
  evidence_kind      evidence_kind_enum NOT NULL,
  artifact_hint      text NOT NULL,
  issued_at          timestamptz NOT NULL DEFAULT now(),
  expires_at         timestamptz,
  used_at            timestamptz,
  state              consent_state_enum NOT NULL DEFAULT 'pending'
);

CREATE INDEX ix_consent_challenges_state ON consent_challenges(state);
CREATE INDEX ix_consent_challenges_expires ON consent_challenges(expires_at);
CREATE INDEX ix_challenges_tuple ON consent_challenges (principal, resource, action, state);

CREATE TABLE consent_receipts (
  consent_id         uuid PRIMARY KEY DEFAULT uuidv7(),
  principal          principal_enum NOT NULL,
  principal_hid      hid_bytes NOT NULL,
  action             consent_action_enum NOT NULL,
  scope              consent_scope_enum[],
  evidence_kind      evidence_kind_enum NOT NULL,
  evidence_url       text NOT NULL,
  evidence_fingerprint text,
  created_at         timestamptz NOT NULL DEFAULT now(),
  last_verified_at   timestamptz,
  revoked_at         timestamptz,
  terms_version      text,
  state              consent_state_enum NOT NULL DEFAULT 'active',
  UNIQUE (principal, principal_hid, action)
);

CREATE INDEX ix_consent_receipts_active ON consent_receipts(principal, state, last_verified_at);
CREATE INDEX ix_consent_receipts_tuple ON consent_receipts (principal, principal_hid, action, state);

CREATE INDEX ix_deny_repo_hid_active
  ON consent_receipts (principal_hid)
  WHERE principal='repo' AND action='opt_out' AND state='active';

CREATE INDEX ix_deny_actor_hid_active
  ON consent_receipts (principal_hid)
  WHERE principal='actor' AND action='opt_out' AND state='active';

CREATE INDEX ix_allow_repo_hid_active
  ON consent_receipts (principal_hid)
  WHERE principal='repo' AND action='opt_in' AND state='active';

CREATE INDEX ix_allow_actor_hid_active
  ON consent_receipts (principal_hid)
  WHERE principal='actor' AND action='opt_in' AND state='active';

CREATE INDEX ix_consent_receipts_scope_gin ON consent_receipts USING gin (scope);

CREATE TABLE repo_directory (
  consent_id   uuid PRIMARY KEY REFERENCES consent_receipts(consent_id) ON DELETE CASCADE,
  repo_id      bigint NOT NULL,
  owner        text   NOT NULL,
  name         text   NOT NULL
);

CREATE UNIQUE INDEX ux_repo_directory_repoid ON repo_directory(repo_id);

CREATE TABLE actor_directory (
  consent_id   uuid PRIMARY KEY REFERENCES consent_receipts(consent_id) ON DELETE CASCADE,
  user_id      bigint NOT NULL,
  login        text   NOT NULL
);

CREATE UNIQUE INDEX ux_actor_directory_userid ON actor_directory(user_id);

-- Effective policy views
CREATE VIEW active_deny_repos AS
  SELECT principal_hid FROM consent_receipts
   WHERE principal = 'repo' AND action = 'opt_out' AND state = 'active';

CREATE VIEW active_deny_actors AS
  SELECT principal_hid FROM consent_receipts
   WHERE principal = 'actor' AND action = 'opt_out' AND state = 'active';

CREATE VIEW active_allow_repos AS
  SELECT r.principal_hid, d.repo_id, d.owner, d.name
    FROM consent_receipts r
    JOIN repo_directory d USING (consent_id)
   WHERE r.principal = 'repo' AND r.action = 'opt_in' AND r.state = 'active';

CREATE VIEW active_allow_actors AS
  SELECT r.principal_hid, d.user_id, d.login
    FROM consent_receipts r
    JOIN actor_directory d USING (consent_id)
   WHERE r.principal = 'actor' AND r.action = 'opt_in' AND r.state = 'active';

-- Ingest accounting
CREATE TABLE ingest_hours (
  hour_utc             timestamptz PRIMARY KEY,
  started_at           timestamptz NOT NULL DEFAULT now(),
  finished_at          timestamptz,
  status               text NOT NULL DEFAULT 'running',
  cache_hit            boolean,
  bytes_uncompressed   bigint,
  events_scanned       int,
  utterances_extracted int,
  inserted             int,
  deduped              int,
  fetch_ms             int,
  read_ms              int,
  db_ms                int,
  elapsed_ms           int,
  error                text,
  dropped_due_to_optouts int,
  policy_reverify_count int,
  policy_reverify_ms    int
);

CREATE INDEX ix_ingest_hours_status ON ingest_hours(status) WHERE finished_at IS NULL;

-- Seed initial rulepack (v1)
-- checksum_sha256 is SHA-256 of the raw rules.json file
INSERT INTO rulepacks (version, description, checksum_sha256)
VALUES
  (
    1,
    'seed: embedded rules.json v1',
    '\x644080b9f56902cb95ce7f58dc6115d33819db135dbffbd1cc0f36f7bbcdcdc7'
  );

-- Table for ingest job leases.
-- single row per hour, claimed_at is when the lease was taken
CREATE TABLE ingest_hours_leases (
  hour_utc timestamptz PRIMARY KEY,
  claimed_at timestamptz NOT NULL DEFAULT NOW()
);

---------------------------------------------------------------------
-- HALL MONITOR: catalog of repositories & actors + queues + rollups
---------------------------------------------------------------------

-- Repositories catalog (coding language + popularity/context)
-- (privacy-first; identities never exposed unless opted-in)
CREATE TABLE repositories (
  repo_id         bigint PRIMARY KEY,            -- GitHub numeric id from events
  full_name       text NOT NULL,                 -- owner/name
  default_branch  text,
  primary_lang    text,                          -- repo.language
  languages       jsonb,                         -- /languages map: lang->bytes
  stars           int,
  forks           int,
  subscribers     int,
  open_issues     int,
  license_key     text,
  is_fork         boolean,
  pushed_at       timestamptz,
  updated_at      timestamptz,
  fetched_at      timestamptz NOT NULL DEFAULT now(),
  next_refresh_at timestamptz,                  -- scheduler hint
  etag            text,                         -- for conditional GETs
  api_url         text,
  opted_in_at     timestamptz                   -- null unless consented
);

CREATE INDEX repositories_primary_lang_idx ON repositories (primary_lang);
CREATE INDEX repositories_next_refresh_idx ON repositories (next_refresh_at);
CREATE INDEX repositories_pushed_at_idx ON repositories (pushed_at DESC);

-- Actors catalog (privacy-first; identities never exposed unless opted-in)
CREATE TABLE actors (
  actor_id        bigint PRIMARY KEY,           -- GitHub numeric id from events
  login           text,
  name            text,
  type            text,                         -- "User" | "Organization"
  company         text,
  location        text,
  bio             text,
  blog            text,
  twitter_username text,
  followers       int,
  following       int,
  public_repos    int,
  public_gists    int,
  created_at      timestamptz,
  updated_at      timestamptz,
  fetched_at      timestamptz NOT NULL DEFAULT now(),
  next_refresh_at timestamptz,
  etag            text,
  api_url         text,
  opted_in_at     timestamptz                   -- null unless consented
);

CREATE UNIQUE INDEX actors_login_idx ON actors (lower(login));
CREATE INDEX actors_next_refresh_idx ON actors (next_refresh_at);

-- Non-blocking queues for hallmonitor worker
CREATE TABLE repo_catalog_queue (
  repo_id         bigint PRIMARY KEY,
  priority        smallint NOT NULL DEFAULT 0,  -- 0=normal, higher=sooner
  attempts        int      NOT NULL DEFAULT 0,
  last_error      text,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  enqueued_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE actor_catalog_queue (
  actor_id        bigint PRIMARY KEY,
  priority        smallint NOT NULL DEFAULT 0,
  attempts        int      NOT NULL DEFAULT 0,
  last_error      text,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  enqueued_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX repo_catalog_queue_due_idx  ON repo_catalog_queue  (next_attempt_at, priority DESC);
CREATE INDEX actor_catalog_queue_due_idx ON actor_catalog_queue (next_attempt_at, priority DESC);

-- Retroactive population helpers (seed queues from existing utterances)
INSERT INTO repo_catalog_queue (repo_id)
SELECT DISTINCT u.repo_id
FROM utterances u
LEFT JOIN repositories r ON r.repo_id = u.repo_id
WHERE r.repo_id IS NULL
ON CONFLICT (repo_id) DO NOTHING;

INSERT INTO actor_catalog_queue (actor_id)
SELECT DISTINCT u.actor_id
FROM utterances u
LEFT JOIN actors a ON a.actor_id = u.actor_id
WHERE a.actor_id IS NULL
ON CONFLICT (actor_id) DO NOTHING;

-- Language-centric aggregates (coding language, not spoken)
CREATE MATERIALIZED VIEW agg_daily_hits_by_code_lang AS
SELECT
  date_trunc('day', u.created_at) AS day,
  COALESCE(r.primary_lang, 'Unknown') AS code_lang,
  count(*) AS hits
FROM hits h
JOIN utterances u ON u.id = h.utterance_id
LEFT JOIN repositories r ON r.repo_id = u.repo_id
GROUP BY 1, 2;

CREATE INDEX agg_dhcl_day_lang_idx ON agg_daily_hits_by_code_lang (day, code_lang);

-- Privacy-preserving actor+code_language rollup (uses actor_hid from utterances)
CREATE MATERIALIZED VIEW agg_daily_hits_by_actor_lang AS
SELECT
  date_trunc('day', u.created_at) AS day,
  COALESCE(r.primary_lang, 'Unknown') AS code_lang,
  u.actor_hid AS actor_hid,
  count(*) AS hits
FROM hits h
JOIN utterances u ON u.id = h.utterance_id
LEFT JOIN repositories r ON r.repo_id = u.repo_id
GROUP BY 1, 2, 3;

CREATE INDEX agg_dhalb_day_lang_idx ON agg_daily_hits_by_actor_lang (day, code_lang);
