-- PG INIT SCRIPT FOR SWEARJAR
-- Run once on DB init
-- @TODO: Migrations system

-- This is heavily a work in progress. Expect breaking changes

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pg_cld2;

-- Enums
CREATE TYPE source_enum         AS ENUM ('commit','issue','pr','comment');
CREATE TYPE hit_category_enum   AS ENUM ('bot_rage','tooling_rage','self_own','generic');
CREATE TYPE hit_severity_enum   AS ENUM ('mild','strong','slur_masked');
CREATE TYPE stop_kind_enum      AS ENUM ('exact','substring','regex');
CREATE TYPE principal_enum      AS ENUM ('repo','actor');
CREATE TYPE consent_action_enum AS ENUM ('opt_in','opt_out');
CREATE TYPE consent_state_enum  AS ENUM ('pending','active','revoked','expired');
CREATE TYPE consent_scope_enum  AS ENUM ('demask_repo','demask_self');
CREATE TYPE evidence_kind_enum  AS ENUM ('repo_file','actor_gist');

-- Domains
CREATE DOMAIN sha256_bytes AS bytea CHECK (octet_length(VALUE) = 32);
CREATE DOMAIN hid_bytes    AS bytea CHECK (octet_length(VALUE) = 32);

-- WAL

ALTER SYSTEM SET max_wal_size = '4GB';                 -- upper bound
ALTER SYSTEM SET min_wal_size = '1GB';                 -- keeps segments around
ALTER SYSTEM SET checkpoint_timeout = '15min';         -- time-based cap
ALTER SYSTEM SET checkpoint_completion_target = 0.9;   -- spread I/O
ALTER SYSTEM SET wal_compression = on;                 -- CPU for less WAL

ALTER SYSTEM SET deadlock_timeout = '200ms';

SELECT pg_reload_conf();  -- no restart needed for these

-- Rulepack meta (versioning)
CREATE TABLE rulepacks (
  version         int PRIMARY KEY,
  created_at      timestamptz NOT NULL DEFAULT now(),
  description     text,
  checksum_sha256 bytea
);

-- CONSENT (Source of truth for all PII/enrichment)

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
CREATE INDEX ix_consent_challenges_state   ON consent_challenges(state);
CREATE INDEX ix_consent_challenges_expires ON consent_challenges(expires_at);
CREATE INDEX ix_challenges_tuple           ON consent_challenges (principal, resource, action, state);

CREATE TABLE consent_receipts (
  consent_id           uuid PRIMARY KEY DEFAULT uuidv7(),
  principal            principal_enum NOT NULL,
  principal_hid        hid_bytes NOT NULL,
  action               consent_action_enum NOT NULL,
  scope                consent_scope_enum[],
  evidence_kind        evidence_kind_enum NOT NULL,
  evidence_url         text NOT NULL,
  evidence_fingerprint text,
  created_at           timestamptz NOT NULL DEFAULT now(),
  last_verified_at     timestamptz,
  revoked_at           timestamptz,
  terms_version        text,
  state                consent_state_enum NOT NULL DEFAULT 'active',
  UNIQUE (principal, principal_hid, action)
);
CREATE INDEX ix_consent_receipts_active    ON consent_receipts(principal, state, last_verified_at);
CREATE INDEX ix_consent_receipts_tuple     ON consent_receipts (principal, principal_hid, action, state);
CREATE INDEX ix_deny_repo_hid_active  ON consent_receipts (principal_hid)
  WHERE principal='repo'  AND action='opt_out' AND state='active';
CREATE INDEX ix_deny_actor_hid_active ON consent_receipts (principal_hid)
  WHERE principal='actor' AND action='opt_out' AND state='active';
CREATE INDEX ix_allow_repo_hid_active ON consent_receipts (principal_hid)
  WHERE principal='repo'  AND action='opt_in'  AND state='active';
CREATE INDEX ix_allow_actor_hid_active ON consent_receipts (principal_hid)
  WHERE principal='actor' AND action='opt_in'  AND state='active';
CREATE INDEX ix_consent_receipts_scope_gin ON consent_receipts USING gin (scope);

-- Effective policy views (opt-in only surfaces identities)
CREATE VIEW active_deny_repos AS
  SELECT principal_hid FROM consent_receipts
   WHERE principal='repo'  AND action='opt_out' AND state='active';

CREATE VIEW active_deny_actors AS
  SELECT principal_hid FROM consent_receipts
   WHERE principal='actor' AND action='opt_out' AND state='active';


-- PRINCIPALS (minimal FK targets)
CREATE TABLE principals_repos  (repo_hid  hid_bytes PRIMARY KEY);
CREATE TABLE principals_actors (actor_hid hid_bytes PRIMARY KEY);

-- ENRICHMENT CATALOGS (optional; keyed by HID, exposed only when opted-in)

CREATE TABLE repositories (
  repo_hid        hid_bytes  PRIMARY KEY REFERENCES principals_repos(repo_hid) ON DELETE CASCADE,
  consent_id      uuid UNIQUE REFERENCES consent_receipts(consent_id) ON DELETE CASCADE,
  full_name       text, -- owner/name (only if opted-in)
  default_branch  text,
  primary_lang    text, -- GitHub repo.language (coding)
  languages       jsonb, -- /languages map: lang -> bytes
  stars           int,
  forks           int,
  subscribers     int,
  open_issues     int,
  license_key     text,
  is_fork         boolean,
  pushed_at       timestamptz,
  updated_at      timestamptz,
  fetched_at      timestamptz NOT NULL DEFAULT now(),
  next_refresh_at timestamptz,
  etag            text,
  api_url         text,
  gone_at         timestamptz,
  gone_code       int2, -- e.g. 404, 410, 451
  gone_reason     text
);
CREATE INDEX repositories_primary_lang_idx ON repositories (primary_lang);
CREATE INDEX repositories_next_refresh_idx ON repositories (next_refresh_at);
CREATE INDEX repositories_pushed_at_idx    ON repositories (pushed_at DESC);
CREATE INDEX repositories_gone_idx         ON repositories (gone_at) WHERE gone_at IS NOT NULL;

CREATE TABLE actors (
  actor_hid        hid_bytes  PRIMARY KEY REFERENCES principals_actors(actor_hid) ON DELETE CASCADE,
  consent_id       uuid UNIQUE REFERENCES consent_receipts(consent_id) ON DELETE CASCADE,
  login            text,
  name             text,
  type             text, -- "User" | "Organization"
  company          text,
  location         text,
  bio              text,
  blog             text,
  twitter_username text,
  followers        int,
  following        int,
  public_repos     int,
  public_gists     int,
  created_at       timestamptz,
  updated_at       timestamptz,
  fetched_at       timestamptz NOT NULL DEFAULT now(),
  next_refresh_at  timestamptz,
  etag             text,
  api_url          text,
  gone_at          timestamptz,
  gone_code        int2, -- e.g. 404, 410, 451
  gone_reason      text
);
CREATE UNIQUE INDEX actors_login_idx  ON actors (lower(login)) WHERE login IS NOT NULL;
CREATE INDEX actors_next_refresh_idx  ON actors (next_refresh_at);
CREATE INDEX actors_gone_idx ON actors (gone_at) WHERE gone_at IS NOT NULL;

-- "Opt-in views" that surface public identifiers for those who consented
CREATE VIEW active_allow_repos AS
  SELECT r.principal_hid, c.repo_hid, c.full_name, c.default_branch
    FROM consent_receipts r
    JOIN repositories c ON c.consent_id = r.consent_id
   WHERE r.principal='repo'  AND r.action='opt_in' AND r.state='active';

CREATE VIEW active_allow_actors AS
  SELECT r.principal_hid, a.actor_hid, a.login, a.name
    FROM consent_receipts r
    JOIN actors a ON a.consent_id = r.consent_id
   WHERE r.principal='actor' AND r.action='opt_in' AND r.state='active';

-- Utterances (CLD2 auto lang)

CREATE TABLE utterances (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  event_id          text        NOT NULL,
  event_type        text        NOT NULL,
  repo_hid          hid_bytes   NOT NULL REFERENCES principals_repos(repo_hid)  DEFERRABLE INITIALLY DEFERRED,
  actor_hid         hid_bytes   NOT NULL REFERENCES principals_actors(actor_hid) DEFERRABLE INITIALLY DEFERRED,
  hid_key_version   smallint    NOT NULL,
  created_at        timestamptz NOT NULL,
  source            source_enum NOT NULL,
  source_detail     text        NOT NULL DEFAULT '',
  ordinal           int         NOT NULL,
  text_raw          text        NOT NULL,
  text_normalized   text,
  -- spoken language (primary only; populated by trigger)
  lang_code         text,
  lang_script       text,
  lang_reliable     boolean,
  lang_confidence   smallint
);
CREATE UNIQUE INDEX ux_utterances_event_source_ord ON utterances(event_id, source, ordinal);
CREATE INDEX ix_utterances_text_norm_trgm          ON utterances USING gin (text_normalized gin_trgm_ops);
CREATE INDEX ix_utterances_repo_hid_time           ON utterances(repo_hid, created_at);
CREATE INDEX ix_utterances_actor_hid_time          ON utterances(actor_hid, created_at);
CREATE INDEX ix_utterances_created_at              ON utterances(created_at);
CREATE INDEX ix_utterances_created_id              ON utterances (created_at, id);
CREATE INDEX ix_utterances_type_time               ON utterances(event_type, created_at);

-- CLD2 trigger
CREATE OR REPLACE FUNCTION trg_set_utterance_language()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r RECORD;
BEGIN
  SELECT * INTO r FROM pg_cld2_detect_language(NEW.text_raw);
  IF FOUND THEN
    NEW.lang_code       := r.mll_language_code;
    NEW.lang_script     := r.mll_primary_script_name;
    NEW.lang_reliable   := r.is_reliable;
    NEW.lang_confidence := CASE
      WHEN r.text_bytes > 0
        THEN GREATEST(0, LEAST(100, FLOOR(100.0 * r.valid_prefix_bytes::numeric / r.text_bytes)))
      ELSE NULL
    END;
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER t_utterances_lang_before_ins
BEFORE INSERT ON utterances
FOR EACH ROW EXECUTE FUNCTION trg_set_utterance_language();

CREATE TRIGGER t_utterances_lang_before_upd
BEFORE UPDATE OF text_raw ON utterances
FOR EACH ROW EXECUTE FUNCTION trg_set_utterance_language();

-- Hits (HID-only)

CREATE TABLE hits (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  utterance_id     uuid NOT NULL REFERENCES utterances(id) ON DELETE CASCADE,
  created_at       timestamptz NOT NULL DEFAULT now(),
  source           source_enum NOT NULL,
  repo_hid         hid_bytes   NOT NULL REFERENCES principals_repos(repo_hid)  DEFERRABLE INITIALLY DEFERRED,
  actor_hid        hid_bytes   NOT NULL REFERENCES principals_actors(actor_hid) DEFERRABLE INITIALLY DEFERRED,
  lang_code        text,
  term             text NOT NULL,
  category         hit_category_enum NOT NULL,
  severity         hit_severity_enum NOT NULL,
  span_start       int NOT NULL,
  span_end         int NOT NULL,
  detector_version int NOT NULL REFERENCES rulepacks(version)
);
ALTER TABLE hits ADD CONSTRAINT ck_hits_span_valid CHECK (span_start >= 0 AND span_end > span_start);

CREATE UNIQUE INDEX uniq_hits_semantic       ON hits (utterance_id, term, span_start, span_end, detector_version);
CREATE INDEX idx_hits_repo_hid_created       ON hits (repo_hid, created_at);
CREATE INDEX idx_hits_actor_hid_created      ON hits (actor_hid, created_at);
CREATE INDEX idx_hits_cat_sev                ON hits (category, severity);
CREATE INDEX idx_hits_lang                   ON hits (lang_code);
CREATE INDEX idx_hits_term_created           ON hits (term, created_at);
CREATE INDEX idx_hits_created_at             ON hits (created_at);
CREATE INDEX idx_hits_detver_created         ON hits (detector_version, created_at);

-- Aggregates (spoken/coding)

CREATE TABLE agg_daily_lang_spk (
  day               date NOT NULL,
  lang_code         text,
  hits              bigint NOT NULL,
  events            bigint NOT NULL,
  detector_version  int NOT NULL REFERENCES rulepacks(version),
  PRIMARY KEY (day, lang_code, detector_version)
);

CREATE MATERIALIZED VIEW agg_daily_hits_by_code_lang AS
SELECT
  date_trunc('day', u.created_at) AS day,
  COALESCE(r.primary_lang, 'Unknown') AS code_lang,
  count(*) AS hits
FROM hits h
JOIN utterances u ON u.id = h.utterance_id
LEFT JOIN repositories r ON r.repo_hid = u.repo_hid
GROUP BY 1, 2;
CREATE INDEX agg_dhcl_day_lang_idx ON agg_daily_hits_by_code_lang (day, code_lang);

CREATE MATERIALIZED VIEW agg_daily_hits_by_actor_lang AS
SELECT
  date_trunc('day', u.created_at) AS day,
  COALESCE(r.primary_lang, 'Unknown') AS code_lang,
  u.actor_hid AS actor_hid,
  count(*) AS hits
FROM hits h
JOIN utterances u ON u.id = h.utterance_id
LEFT JOIN repositories r ON r.repo_hid = u.repo_hid
GROUP BY 1, 2, 3;
CREATE INDEX agg_dhalb_day_lang_idx ON agg_daily_hits_by_actor_lang (day, code_lang);

-- Ingest accounting + leases

CREATE TABLE ingest_hours (
  hour_utc               timestamptz PRIMARY KEY,
  started_at             timestamptz NOT NULL DEFAULT now(),
  finished_at            timestamptz,
  status                 text NOT NULL DEFAULT 'running',
  cache_hit              boolean,
  bytes_uncompressed     bigint,
  events_scanned         int,
  utterances_extracted   int,
  inserted               int,
  deduped                int,
  fetch_ms               int,
  read_ms                int,
  db_ms                  int,
  elapsed_ms             int,
  error                  text,
  dropped_due_to_optouts int,
  policy_reverify_count  int,
  policy_reverify_ms     int
);
CREATE INDEX ix_ingest_hours_status ON ingest_hours(status) WHERE finished_at IS NULL;

INSERT INTO rulepacks (version, description, checksum_sha256) VALUES
(1, 'seed: embedded rules.json v1', '\x644080b9f56902cb95ce7f58dc6115d33819db135dbffbd1cc0f36f7bbcdcdc7');

CREATE TABLE ingest_hours_leases (
  hour_utc   timestamptz PRIMARY KEY,
  claimed_at timestamptz NOT NULL DEFAULT now()
);

-- Hallmonitor queues (HID keyed)

CREATE TABLE repo_catalog_queue (
  repo_hid        hid_bytes PRIMARY KEY REFERENCES principals_repos(repo_hid) ON DELETE CASCADE,
  priority        smallint NOT NULL DEFAULT 0,
  attempts        int      NOT NULL DEFAULT 0,
  last_error      text,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  enqueued_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX repo_catalog_queue_due_idx  ON repo_catalog_queue  (next_attempt_at, priority DESC);

CREATE TABLE actor_catalog_queue (
  actor_hid       hid_bytes PRIMARY KEY REFERENCES principals_actors(actor_hid) ON DELETE CASCADE,
  priority        smallint NOT NULL DEFAULT 0,
  attempts        int      NOT NULL DEFAULT 0,
  last_error      text,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  enqueued_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX actor_catalog_queue_due_idx ON actor_catalog_queue (next_attempt_at, priority DESC);

-- Seed queues from utterances (safe even empty)
INSERT INTO repo_catalog_queue (repo_hid)
SELECT DISTINCT u.repo_hid
FROM utterances u
LEFT JOIN repositories r ON r.repo_hid = u.repo_hid
WHERE r.repo_hid IS NULL
ON CONFLICT (repo_hid) DO NOTHING;

INSERT INTO actor_catalog_queue (actor_hid)
SELECT DISTINCT u.actor_hid
FROM utterances u
LEFT JOIN actors a ON a.actor_hid = u.actor_hid
WHERE a.actor_hid IS NULL
ON CONFLICT (actor_hid) DO NOTHING;

-- IDENT schema (maps + audit)

CREATE SCHEMA IF NOT EXISTS ident;

CREATE TABLE ident.gh_repo_map (
  repo_hid   hid_bytes PRIMARY KEY,
  gh_repo_id bigint      NOT NULL,
  seen_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ident.gh_actor_map (
  actor_hid  hid_bytes PRIMARY KEY,
  gh_user_id bigint      NOT NULL,
  seen_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ident.identity_resolve_audit (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  ts            timestamptz NOT NULL DEFAULT now(),
  who           text        NOT NULL, -- service principal
  purpose       text        NOT NULL, -- "hallmonitor_refresh", etc.
  principal     principal_enum NOT NULL,
  principal_hid hid_bytes   NOT NULL,
  allowed       boolean     NOT NULL,
  reason        text
);

-- Consent helpers

CREATE OR REPLACE FUNCTION can_expose_repo(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
      FROM consent_receipts r
     WHERE r.principal='repo' AND r.principal_hid = $1
       AND r.action='opt_in' AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1
      FROM consent_receipts r
     WHERE r.principal='repo' AND r.principal_hid = $1
       AND r.action='opt_out' AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_expose_actor(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
      FROM consent_receipts r
     WHERE r.principal='actor' AND r.principal_hid = $1
       AND r.action='opt_in' AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1
      FROM consent_receipts r
     WHERE r.principal='actor' AND r.principal_hid = $1
       AND r.action='opt_out' AND r.state='active'
  );
$$;

-- SECURITY INVOKER API (consent-gated)
-- NOTE: we rely on search_path pinning

CREATE OR REPLACE FUNCTION ident.resolve_repo(p_hid hid_bytes, p_who text, p_purpose text)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ident, public
AS $$
DECLARE
  gh_id bigint;
  ok    boolean;
  reason text;
BEGIN
  ok := can_fetch_repo(p_hid);
  IF ok THEN
    SELECT gh_repo_id INTO gh_id FROM gh_repo_map WHERE repo_hid = p_hid;
    reason := CASE WHEN gh_id IS NULL
                   THEN 'allowed:no_optout,map_miss'
                   ELSE 'allowed:no_optout'
              END;
    INSERT INTO identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'repo', p_hid, true, reason);
    RETURN gh_id; -- may be NULL if not mapped yet
  ELSE
    INSERT INTO identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'repo', p_hid, false, 'denied:optout_active');
    RETURN NULL;
  END IF;
END $$;

CREATE OR REPLACE FUNCTION ident.resolve_actor(p_hid hid_bytes, p_who text, p_purpose text)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ident, public
AS $$
DECLARE
  gh_id bigint;
  ok    boolean;
  reason text;
BEGIN
  ok := can_fetch_actor(p_hid);
  IF ok THEN
    SELECT gh_user_id INTO gh_id FROM gh_actor_map WHERE actor_hid = p_hid;
    reason := CASE WHEN gh_id IS NULL
                   THEN 'allowed:no_optout,map_miss'
                   ELSE 'allowed:no_optout'
              END;
    INSERT INTO identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'actor', p_hid, true, reason);
    RETURN gh_id;
  ELSE
    INSERT INTO identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'actor', p_hid, false, 'denied:optout_active');
    RETURN NULL;
  END IF;
END $$;

-- Helpers to read public catalog rows only when allowed:
CREATE OR REPLACE FUNCTION ident.repo_public(p_hid hid_bytes)
RETURNS repositories
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ident, public
AS $$
DECLARE rec repositories%ROWTYPE;
BEGIN
  IF can_expose_repo(p_hid) THEN
    SELECT * INTO rec FROM repositories WHERE repo_hid = p_hid;
    RETURN rec;
  END IF;
  RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION ident.actor_public(p_hid hid_bytes)
RETURNS actors
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ident, public
AS $$
DECLARE rec actors%ROWTYPE;
BEGIN
  IF can_expose_actor(p_hid) THEN
    SELECT * INTO rec FROM actors WHERE actor_hid = p_hid;
    RETURN rec;
  END IF;
  RETURN NULL;
END $$;

-- PURGE on active OPT-OUT (delete everything we can)

CREATE OR REPLACE FUNCTION trg_purge_on_optout()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  -- only act when action=opt_out becomes active
  IF NEW.action='opt_out' AND NEW.state='active' THEN
    IF NEW.principal='repo' THEN
      -- delete derived first, then facts, then enrichment + maps + queues
      DELETE FROM hits        WHERE repo_hid  = NEW.principal_hid;
      DELETE FROM utterances  WHERE repo_hid  = NEW.principal_hid;
      DELETE FROM repositories WHERE repo_hid = NEW.principal_hid;
      DELETE FROM repo_catalog_queue WHERE repo_hid = NEW.principal_hid;
      DELETE FROM ident.gh_repo_map   WHERE repo_hid = NEW.principal_hid;
      -- principals row left in place; ingest path should skip via deny view
    ELSIF NEW.principal='actor' THEN
      DELETE FROM hits        WHERE actor_hid = NEW.principal_hid;
      DELETE FROM utterances  WHERE actor_hid = NEW.principal_hid;
      DELETE FROM actors      WHERE actor_hid = NEW.principal_hid;
      DELETE FROM actor_catalog_queue WHERE actor_hid = NEW.principal_hid;
      DELETE FROM ident.gh_actor_map  WHERE actor_hid = NEW.principal_hid;
    END IF;
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER t_purge_on_optout_ins AFTER INSERT ON consent_receipts FOR EACH ROW EXECUTE FUNCTION trg_purge_on_optout();
CREATE TRIGGER t_purge_on_optout_upd AFTER UPDATE OF state ON consent_receipts FOR EACH ROW EXECUTE FUNCTION trg_purge_on_optout();

-- Define consent helpers (needed by the CHECKs below)
CREATE OR REPLACE FUNCTION can_expose_repo(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1 FROM consent_receipts r WHERE r.principal='repo' AND r.principal_hid=$1 AND r.action='opt_in'  AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1 FROM consent_receipts r WHERE r.principal='repo' AND r.principal_hid=$1 AND r.action='opt_out' AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_expose_actor(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1 FROM consent_receipts r WHERE r.principal='actor' AND r.principal_hid=$1 AND r.action='opt_in'  AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1 FROM consent_receipts r WHERE r.principal='actor' AND r.principal_hid=$1 AND r.action='opt_out' AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_fetch_repo(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT NOT EXISTS (
    SELECT 1 FROM consent_receipts r
     WHERE r.principal='repo' AND r.principal_hid=$1
       AND r.action='opt_out' AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_fetch_actor(hid hid_bytes)
RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT NOT EXISTS (
    SELECT 1 FROM consent_receipts r
     WHERE r.principal='actor' AND r.principal_hid=$1
       AND r.action='opt_out' AND r.state='active'
  );
$$;

-- Principals: add URL id + private explicit label + live label (no triggers)
ALTER TABLE principals_repos
  ADD COLUMN hid_hex text
    GENERATED ALWAYS AS (lower(encode(repo_hid,'hex'))) STORED,
  ADD COLUMN _label_explicit text,
  ADD COLUMN label text
    GENERATED ALWAYS AS (
      COALESCE(
        _label_explicit,
        substr(lower(encode(repo_hid,'hex')),1,6) || '...' ||
        substr(lower(encode(repo_hid,'hex')),59,6)
      )
    ) STORED;

ALTER TABLE principals_actors
  ADD COLUMN hid_hex text
    GENERATED ALWAYS AS (lower(encode(actor_hid,'hex'))) STORED,
  ADD COLUMN _label_explicit text,
  ADD COLUMN label text
    GENERATED ALWAYS AS (
      COALESCE(
        _label_explicit,
        substr(lower(encode(actor_hid,'hex')),1,6) || '...' ||
        substr(lower(encode(actor_hid,'hex')),59,6)
      )
    ) STORED;

-- Guardrails: explicit label only when consent says we may expose
ALTER TABLE principals_repos ADD CONSTRAINT ck_repo_label_consent CHECK (_label_explicit IS NULL OR can_expose_repo(repo_hid));
ALTER TABLE principals_actors ADD CONSTRAINT ck_actor_label_consent CHECK (_label_explicit IS NULL OR can_expose_actor(actor_hid));

-- Indexes for API and search by label/hex
CREATE UNIQUE INDEX IF NOT EXISTS principals_repos_hidhex_idx ON principals_repos(hid_hex);
CREATE UNIQUE INDEX IF NOT EXISTS principals_actors_hidhex_idx ON principals_actors(hid_hex);
CREATE INDEX IF NOT EXISTS principals_repos_label_ci_idx ON principals_repos(lower(label));
CREATE INDEX IF NOT EXISTS principals_actors_label_ci_idx ON principals_actors(lower(label));

--
-- APP ROLE GRANTS
-- sw_api: full RW on public; no direct table access in ident; can exec upsert shims
-- sw_hallmonitor: full RW on public; SELECT in ident; can exec resolvers/public + shims
--
-- On PG18 ON CONFLICT DO NOTHING is treated as a read-revealing operation. We use a tiny SECURITY DEFINER upsert shim
--

-- name resolution for ident
GRANT USAGE ON SCHEMA ident TO sw_api, sw_hallmonitor;

-- ident tables: no direct access for sw_api; read-only for hallmonitor
REVOKE ALL ON ALL TABLES IN SCHEMA ident FROM sw_api, sw_hallmonitor;
GRANT SELECT ON ALL TABLES IN SCHEMA ident TO sw_hallmonitor;

-- lock down ident functions (PUBLIC gets nothing)
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA ident FROM PUBLIC, sw_api, sw_hallmonitor;

-- allow consent-gated resolvers/readers only to hallmonitor
GRANT EXECUTE ON FUNCTION ident.resolve_repo(hid_bytes, text, text)  TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.resolve_actor(hid_bytes, text, text) TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.repo_public(hid_bytes) TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.actor_public(hid_bytes) TO sw_hallmonitor;

-- public: both roles full RW + sequences + function execute
GRANT USAGE ON SCHEMA public TO sw_api, sw_hallmonitor;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO sw_api, sw_hallmonitor;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO sw_api, sw_hallmonitor;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO sw_api, sw_hallmonitor;

-- defaults for future public objects (created by the role running this)
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sw_api, sw_hallmonitor;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO sw_api, sw_hallmonitor;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO sw_api, sw_hallmonitor;

-- owner must be a trusted role with full rights on ident.* (e.g., swearjarbot)
CREATE OR REPLACE FUNCTION ident.upsert_gh_repo_map(p_hid hid_bytes, p_gh_id bigint)
RETURNS void
LANGUAGE sql SECURITY DEFINER
SET search_path = ident, public
AS $$
  INSERT INTO gh_repo_map (repo_hid, gh_repo_id) VALUES (p_hid, p_gh_id) ON CONFLICT (repo_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint) OWNER TO swearjarbot;
REVOKE ALL ON FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint) FROM PUBLIC;

CREATE OR REPLACE FUNCTION ident.upsert_gh_actor_map(p_hid hid_bytes, p_gh_id bigint)
RETURNS void
LANGUAGE sql SECURITY DEFINER
SET search_path = ident, public
AS $$
  INSERT INTO gh_actor_map (actor_hid, gh_user_id) VALUES (p_hid, p_gh_id) ON CONFLICT (actor_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) OWNER TO swearjarbot;
REVOKE ALL ON FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) FROM PUBLIC;

-- allow shims to both roles (insert via definer; no SELECT needed)
GRANT EXECUTE ON FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint)  TO sw_api, sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) TO sw_api, sw_hallmonitor;

-- Create a "broker" role for cross-service access to consent-gated functions + tables
-- This role should not be directly logged into
DO $$BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='broker') THEN
    CREATE ROLE broker NOLOGIN;
  END IF;
END$$;

-- Make sure the consent-gated functions are owned by broker
ALTER FUNCTION ident.resolve_repo(hid_bytes, text, text)  OWNER TO broker;
ALTER FUNCTION ident.resolve_actor(hid_bytes, text, text) OWNER TO broker;
ALTER FUNCTION ident.repo_public(hid_bytes) OWNER TO broker;
ALTER FUNCTION ident.actor_public(hid_bytes) OWNER TO broker;

-- Broker needs schema visibility
GRANT USAGE ON SCHEMA ident  TO broker;
GRANT USAGE ON SCHEMA public TO broker;

-- Minimal table privileges needed BY the SECDEF bodies
GRANT INSERT ON ident.identity_resolve_audit TO broker;
GRANT SELECT ON ident.gh_repo_map, ident.gh_actor_map TO broker;
GRANT SELECT ON public.consent_receipts TO broker;
GRANT SELECT ON public.repositories, public.actors TO broker;


CREATE OR REPLACE FUNCTION ident.bulk_upsert_gh_repo_map(p_hex text[], p_ids bigint[])
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = ident, public
AS $$
  WITH data AS (
    SELECT decode(x,'hex')::hid_bytes AS hid, i::bigint AS id
    FROM unnest(p_hex, p_ids) AS t(x,i)
  ),
  missing AS (
    SELECT d.hid, d.id
    FROM data d
    LEFT JOIN gh_repo_map g ON g.repo_hid = d.hid
    WHERE g.repo_hid IS NULL
  )
  INSERT INTO gh_repo_map (repo_hid, gh_repo_id)
  SELECT hid, id FROM missing
  ON CONFLICT (repo_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.bulk_upsert_gh_repo_map(text[], bigint[]) OWNER TO broker;
REVOKE ALL ON FUNCTION ident.bulk_upsert_gh_repo_map(text[], bigint[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ident.bulk_upsert_gh_repo_map(text[], bigint[]) TO sw_api, sw_hallmonitor;

CREATE OR REPLACE FUNCTION ident.bulk_upsert_gh_actor_map(p_hex text[], p_ids bigint[])
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = ident, public
AS $$
  WITH data AS (
    SELECT decode(x,'hex')::hid_bytes AS hid, i::bigint AS id
    FROM unnest(p_hex, p_ids) AS t(x,i)
  ),
  missing AS (
    SELECT d.hid, d.id
    FROM data d
    LEFT JOIN gh_actor_map g ON g.actor_hid = d.hid
    WHERE g.actor_hid IS NULL
  )
  INSERT INTO gh_actor_map (actor_hid, gh_user_id)
  SELECT hid, id FROM missing
  ON CONFLICT (actor_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) OWNER TO broker;
REVOKE ALL ON FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) TO sw_api, sw_hallmonitor;

GRANT INSERT, SELECT ON ident.gh_repo_map  TO broker;
GRANT INSERT, SELECT ON ident.gh_actor_map TO broker;
