-- PG INIT SCRIPT FOR SWEARJAR
-- Run once on DB init
-- @TODO: Migrations system

-- This is heavily a work in progress. Expect breaking changes

-- =========
-- Extensions
-- =========
CREATE EXTENSION "uuid-ossp";

-- =========
-- Enums
-- =========
-- CREATE TYPE source_enum         AS ENUM ('commit','issue','pr','comment');
-- CREATE TYPE hit_category_enum   AS ENUM ('bot_rage','tooling_rage','self_own','generic');
-- CREATE TYPE hit_severity_enum   AS ENUM ('mild','strong','slur_masked');
-- CREATE TYPE stop_kind_enum      AS ENUM ('exact','substring','regex');

CREATE TYPE principal_enum      AS ENUM ('repo','actor');
CREATE TYPE consent_action_enum AS ENUM ('opt_in','opt_out');
CREATE TYPE consent_state_enum  AS ENUM ('pending','active','revoked','expired');
CREATE TYPE consent_scope_enum  AS ENUM ('demask_repo','demask_self');
CREATE TYPE evidence_kind_enum  AS ENUM ('repo_file','actor_gist');

-- =========
-- Domains
-- =========
CREATE DOMAIN sha256_bytes AS bytea CHECK (octet_length(VALUE) = 32);
CREATE DOMAIN hid_bytes    AS bytea CHECK (octet_length(VALUE) = 32);

-- =========
-- WAL + runtime tuning (instance-level; adjust to your env)
-- =========
ALTER SYSTEM SET max_wal_size = '4GB';
ALTER SYSTEM SET min_wal_size = '1GB';
ALTER SYSTEM SET checkpoint_timeout = '15min';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_compression = on;
ALTER SYSTEM SET deadlock_timeout = '200ms';
SELECT pg_reload_conf();

-- =========
-- Rulepack meta (versioning)
-- =========
CREATE TABLE rulepacks (
  version         int PRIMARY KEY,
  created_at      timestamptz NOT NULL DEFAULT now(),
  description     text,
  checksum_sha256 bytea
);

-- =========
-- CONSENT (Source of truth for all PII [Personally Identifying Information]/enrichment policy)
-- =========
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

-- High-signal verification queue (user-triggered; used when API limits are tight)
CREATE TABLE consent_verifications (
  job_id           uuid PRIMARY KEY DEFAULT uuidv7(),
  principal        principal_enum NOT NULL,
  resource         text NOT NULL,
  principal_hid    hid_bytes NOT NULL,
  challenge_hash   text REFERENCES consent_challenges(challenge_hash) ON DELETE CASCADE,
  attempts         int NOT NULL DEFAULT 0,
  last_error       text,
  last_status      int,
  last_url         text,
  etag_branch      text,
  etag_file        text,
  etag_gists       text,
  rate_reset_at    timestamptz,
  next_attempt_at  timestamptz NOT NULL DEFAULT now(),
  leased_by        text,
  lease_expires_at timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX consent_verifications_subject_ux ON consent_verifications (principal, resource, challenge_hash);
CREATE INDEX consent_verifications_ready_idx ON consent_verifications (next_attempt_at) WHERE leased_by IS NULL;
CREATE INDEX consent_verifications_subject_idx ON consent_verifications (principal, resource);

-- =========
-- PRINCIPALS (minimal FK targets)
-- =========
CREATE TABLE principals_repos  (repo_hid  hid_bytes PRIMARY KEY);
CREATE TABLE principals_actors (actor_hid hid_bytes PRIMARY KEY);

-- =========
-- ENRICHMENT CATALOGS (HID-keyed; PII filled only when opted-in)
-- =========
CREATE TABLE repositories (
  repo_hid        hid_bytes  PRIMARY KEY REFERENCES principals_repos(repo_hid) ON DELETE CASCADE,
  consent_id      uuid UNIQUE REFERENCES consent_receipts(consent_id) ON DELETE CASCADE,
  full_name       text,
  default_branch  text,
  primary_lang    text,
  languages       jsonb,
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
  gone_code       int2,
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
  type             text,
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
  gone_code        int2,
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

-- =========
-- Ingest accounting + leases
-- =========
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

-- =========
-- Hallmonitor queues (HID keyed)
-- =========
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

-- =========
-- IDENT schema (maps + audit)
-- =========
CREATE SCHEMA ident;

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

-- =========
-- Consent helpers (public schema)
-- =========
CREATE OR REPLACE FUNCTION can_expose_repo(hid public.hid_bytes)
RETURNS boolean
LANGUAGE sql
STABLE
SET search_path = pg_temp, public
AS $$
  SELECT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='repo'
      AND r.principal_hid = $1
      AND r.action='opt_in'
      AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='repo'
      AND r.principal_hid = $1
      AND r.action='opt_out'
      AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_expose_actor(hid public.hid_bytes)
RETURNS boolean
LANGUAGE sql
STABLE
SET search_path = pg_temp, public
AS $$
  SELECT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='actor'
      AND r.principal_hid = $1
      AND r.action='opt_in'
      AND r.state='active'
  ) AND NOT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='actor'
      AND r.principal_hid = $1
      AND r.action='opt_out'
      AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_fetch_repo(hid public.hid_bytes)
RETURNS boolean
LANGUAGE sql
STABLE
SET search_path = pg_temp, public
AS $$
  SELECT NOT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='repo'
      AND r.principal_hid=$1
      AND r.action='opt_out'
      AND r.state='active'
  );
$$;

CREATE OR REPLACE FUNCTION can_fetch_actor(hid public.hid_bytes)
RETURNS boolean
LANGUAGE sql
STABLE
SET search_path = pg_temp, public
AS $$
  SELECT NOT EXISTS (
    SELECT 1
    FROM public.consent_receipts r
    WHERE r.principal='actor'
      AND r.principal_hid=$1
      AND r.action='opt_out'
      AND r.state='active'
  );
$$;

-- =========
-- SECURITY DEFINER API (consent-gated) - hardened search_path + qualified refs
-- =========
CREATE OR REPLACE FUNCTION ident.resolve_repo(p_hid hid_bytes, p_who text, p_purpose text)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
DECLARE
  gh_id  bigint;
  ok     boolean;
  reason text;
BEGIN
  ok := public.can_fetch_repo(p_hid);
  IF ok THEN
    SELECT g.gh_repo_id INTO gh_id FROM ident.gh_repo_map AS g WHERE g.repo_hid = p_hid;
    reason := CASE WHEN gh_id IS NULL
                   THEN 'allowed:no_optout,map_miss'
                   ELSE 'allowed:no_optout'
              END;
    INSERT INTO ident.identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'repo', p_hid, true, reason);
    RETURN gh_id; -- may be NULL if not mapped yet
  ELSE
    INSERT INTO ident.identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'repo', p_hid, false, 'denied:optout_active');
    RETURN NULL;
  END IF;
END $$;

CREATE OR REPLACE FUNCTION ident.resolve_actor(p_hid hid_bytes, p_who text, p_purpose text)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
DECLARE
  gh_id  bigint;
  ok     boolean;
  reason text;
BEGIN
  ok := public.can_fetch_actor(p_hid);
  IF ok THEN
    SELECT g.gh_user_id INTO gh_id FROM ident.gh_actor_map AS g WHERE g.actor_hid = p_hid;
    reason := CASE WHEN gh_id IS NULL
                   THEN 'allowed:no_optout,map_miss'
                   ELSE 'allowed:no_optout'
              END;
    INSERT INTO ident.identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'actor', p_hid, true, reason);
    RETURN gh_id;
  ELSE
    INSERT INTO ident.identity_resolve_audit (who, purpose, principal, principal_hid, allowed, reason)
    VALUES (p_who, p_purpose, 'actor', p_hid, false, 'denied:optout_active');
    RETURN NULL;
  END IF;
END $$;

-- Helpers to read public catalog rows only when allowed:
CREATE OR REPLACE FUNCTION ident.repo_public(p_hid hid_bytes)
RETURNS repositories
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
DECLARE rec public.repositories%ROWTYPE;
BEGIN
  IF public.can_expose_repo(p_hid) THEN
    SELECT * INTO rec FROM public.repositories WHERE repo_hid = p_hid;
    RETURN rec;
  END IF;
  RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION ident.actor_public(p_hid hid_bytes)
RETURNS actors
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
DECLARE rec public.actors%ROWTYPE;
BEGIN
  IF public.can_expose_actor(p_hid) THEN
    SELECT * INTO rec FROM public.actors WHERE actor_hid = p_hid;
    RETURN rec;
  END IF;
  RETURN NULL;
END $$;

-- =========
-- PURGE on active OPT-OUT (delete everything we can)
-- note: Hits and Utterances are in CH, and will have to be purged there via separate process
-- =========
CREATE OR REPLACE FUNCTION trg_purge_on_optout()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  -- only act when action=opt_out becomes active
  IF NEW.action='opt_out' AND NEW.state='active' THEN
    IF NEW.principal='repo' THEN
      DELETE FROM repositories         WHERE repo_hid  = NEW.principal_hid;
      DELETE FROM repo_catalog_queue   WHERE repo_hid  = NEW.principal_hid;
      DELETE FROM ident.gh_repo_map    WHERE repo_hid  = NEW.principal_hid;
      -- principals row left in place; ingest path should skip via deny checks
    ELSIF NEW.principal='actor' THEN
      DELETE FROM actors               WHERE actor_hid = NEW.principal_hid;
      DELETE FROM actor_catalog_queue  WHERE actor_hid = NEW.principal_hid;
      DELETE FROM ident.gh_actor_map   WHERE actor_hid = NEW.principal_hid;
    END IF;
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER t_purge_on_optout_ins AFTER INSERT ON consent_receipts
FOR EACH ROW EXECUTE FUNCTION trg_purge_on_optout();

CREATE TRIGGER t_purge_on_optout_upd AFTER UPDATE OF state ON consent_receipts
FOR EACH ROW EXECUTE FUNCTION trg_purge_on_optout();

-- =========
-- Principals: URL id + labels (+ guardrails)
-- =========
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

ALTER TABLE principals_repos  ADD CONSTRAINT ck_repo_label_consent  CHECK (_label_explicit IS NULL OR can_expose_repo(repo_hid));
ALTER TABLE principals_actors ADD CONSTRAINT ck_actor_label_consent CHECK (_label_explicit IS NULL OR can_expose_actor(actor_hid));

CREATE UNIQUE INDEX principals_repos_hidhex_idx  ON principals_repos(hid_hex);
CREATE UNIQUE INDEX principals_actors_hidhex_idx ON principals_actors(hid_hex);
CREATE INDEX principals_repos_label_ci_idx ON principals_repos(lower(label));
CREATE INDEX principals_actors_label_ci_idx ON principals_actors(lower(label));

-- =========
-- APP ROLE GRANTS
-- =========
-- sw_api: full RW on public; no direct table access in ident; can exec upsert shims
-- sw_hallmonitor: full RW on public; SELECT in ident; can exec resolvers/public + shims

-- Harden schema-level surface: block arbitrary CREATE in public by random roles
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- Name resolution for ident
GRANT USAGE ON SCHEMA ident TO sw_api, sw_hallmonitor;

-- ident tables: no direct access for sw_api; read-only for hallmonitor
REVOKE ALL ON ALL TABLES IN SCHEMA ident FROM sw_api, sw_hallmonitor;
GRANT SELECT ON ALL TABLES IN SCHEMA ident TO sw_hallmonitor;

-- Lock down ident functions (PUBLIC gets nothing)
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA ident FROM PUBLIC, sw_api, sw_hallmonitor;

-- Allow consent-gated resolvers/readers only to hallmonitor
GRANT EXECUTE ON FUNCTION ident.resolve_repo(hid_bytes, text, text)  TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.resolve_actor(hid_bytes, text, text) TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.repo_public(hid_bytes)               TO sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.actor_public(hid_bytes)              TO sw_hallmonitor;

-- public: both roles full RW + sequences + function execute
GRANT USAGE ON SCHEMA public TO sw_api, sw_hallmonitor;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO sw_api, sw_hallmonitor;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO sw_api, sw_hallmonitor;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO sw_api, sw_hallmonitor;

-- defaults for future public objects (created by the role running this)
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sw_api, sw_hallmonitor;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO sw_api, sw_hallmonitor;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO sw_api, sw_hallmonitor;

-- =========
-- Upsert shims (SECDEF; owner is trusted; pinned search_path)
-- =========
CREATE OR REPLACE FUNCTION ident.upsert_gh_repo_map(p_hid hid_bytes, p_gh_id bigint)
RETURNS void
LANGUAGE sql SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
  INSERT INTO ident.gh_repo_map (repo_hid, gh_repo_id)
  VALUES (p_hid, p_gh_id)
  ON CONFLICT (repo_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint) OWNER TO swearjarbot;
REVOKE ALL ON FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint) FROM PUBLIC;

CREATE OR REPLACE FUNCTION ident.upsert_gh_actor_map(p_hid hid_bytes, p_gh_id bigint)
RETURNS void
LANGUAGE sql SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
  INSERT INTO ident.gh_actor_map (actor_hid, gh_user_id)
  VALUES (p_hid, p_gh_id)
  ON CONFLICT (actor_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) OWNER TO swearjarbot;
REVOKE ALL ON FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) FROM PUBLIC;

-- allow shims to both roles (insert via definer; no SELECT needed)
GRANT EXECUTE ON FUNCTION ident.upsert_gh_repo_map(hid_bytes, bigint)  TO sw_api, sw_hallmonitor;
GRANT EXECUTE ON FUNCTION ident.upsert_gh_actor_map(hid_bytes, bigint) TO sw_api, sw_hallmonitor;

-- =========
-- Broker role for SECDEF function ownership
-- =========
DO $$BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='broker') THEN
    CREATE ROLE broker NOLOGIN;
  END IF;
END$$;

-- Ensure consent-gated functions are owned by broker
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

-- Ensure broker can execute the public helper functions invoked inside SECDEF
GRANT EXECUTE ON FUNCTION public.can_fetch_repo(hid_bytes)  TO broker;
GRANT EXECUTE ON FUNCTION public.can_fetch_actor(hid_bytes) TO broker;
GRANT EXECUTE ON FUNCTION public.can_expose_repo(hid_bytes) TO broker;
GRANT EXECUTE ON FUNCTION public.can_expose_actor(hid_bytes) TO broker;

-- =========
-- Bulk upserts (SECDEF; pinned search_path; owned by broker)
-- =========
CREATE OR REPLACE FUNCTION ident.bulk_upsert_gh_repo_map(p_hex text[], p_ids bigint[])
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_temp, ident
AS $$
  WITH data AS (
    SELECT decode(x,'hex')::public.hid_bytes AS hid, i::bigint AS id
    FROM unnest(p_hex, p_ids) AS t(x,i)
  ),
  missing AS (
    SELECT d.hid, d.id
    FROM data d
    LEFT JOIN ident.gh_repo_map g ON g.repo_hid = d.hid
    WHERE g.repo_hid IS NULL
  )
  INSERT INTO ident.gh_repo_map (repo_hid, gh_repo_id)
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
SET search_path = pg_temp, ident
AS $$
  WITH data AS (
    SELECT decode(x,'hex')::public.hid_bytes AS hid, i::bigint AS id
    FROM unnest(p_hex, p_ids) AS t(x,i)
  ),
  missing AS (
    SELECT d.hid, d.id
    FROM data d
    LEFT JOIN ident.gh_actor_map g ON g.actor_hid = d.hid
    WHERE g.actor_hid IS NULL
  )
  INSERT INTO ident.gh_actor_map (actor_hid, gh_user_id)
  SELECT hid, id FROM missing
  ON CONFLICT (actor_hid) DO NOTHING;
$$;
ALTER FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) OWNER TO broker;
REVOKE ALL ON FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ident.bulk_upsert_gh_actor_map(text[], bigint[]) TO sw_api, sw_hallmonitor;

GRANT INSERT, SELECT ON ident.gh_repo_map TO broker;
GRANT INSERT, SELECT ON ident.gh_actor_map TO broker;
