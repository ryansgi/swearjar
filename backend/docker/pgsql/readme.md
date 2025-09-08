# Design Philosophy (Actors/Repos)

We keep two kinds of rows on purpose-principals (skeletal identity) and catalog/enrichment (optional, consent-gated details).

## Why separate principals\_\* from repositories/actors?

**Privacy by design / data minimization**
principals\_\* are just HIDs, which are essentially pseudonyms. Safe to create at ingest time, even pre-consent. Repositories/Actors hold anything that could drift into PII (names, logins, locations, etag, cadence...), so they live in a different table that we only touch when policy allows.

**Decoupled ingest vs. enrichment**
Ingest needs FK targets immediately; enrichment is async. With tiny principals\_\* we can insert utterances/hits right away and enqueue for catalog later. We don't have to create "empty" repo/actor rows just to satisfy FKs.

**Clean purge semantics**
On opt-out we nuke enrichment (repositories/actors, maps, queues) and keep the principal row (and HID) so dedup/denylist still work and we don't orphan facts during processing.

**Safer permissions**
Grants can be coarse: everyone can use public (facts + principals); ident and enrichment writers can be tightly scoped. Keeping the sensitive bits out of the FK target lowers the blast radius.

**Operationally lighter**
Principals are narrow, stable FK targets. Enrichment tables churn (etag, counters, next_refresh_at). Separating reduces lock contention and bloat on the thing every fact points at.

**Testing and idempotency**
We can INSERT ... ON CONFLICT DO NOTHING principals cheaply and deterministically on first sighting, without touching consent-gated state.

## Could we merge them?

Yes, but we'd have to re-create the guardrails manually:

- Keep one table keyed by HID that's always present.
- Add CHECKs/row-level guards to ensure PII columns stay NULL unless a matching active opt-in exists.
- Change purge to clear PII + maps (not delete the row) and keep denylist logic.
- Make sweeps/seeders treat "row exists but enrichment empty" as "needs fetch".
- Revisit perms so API can't accidentally see/modify PII columns.

It's all doable, but more prone to foot-guns. The split keeps the hot path simple and the privacy boundary obvious.

# Overview

Generally speaking we run with three roles plus a superuser to protect accidental data leaks and keep PII (Personally Identifiable Information) behind rigid consent gateways.

- **`sw_api`** - used by the API.
  _Access:_ full read/write in `public`; **no direct access** to `ident` tables.
  _Writes to `ident` only via shims:_ `ident.upsert_gh_repo_map(...)`, `ident.upsert_gh_actor_map(...)`. This avoids the PG18 "`ON CONFLICT` implies read" foot-gun and keeps API blind to identity data.

- **`sw_hallmonitor`** - used by the hall monitor.
  _Access:_ same as `sw_api` in `public`, **plus** `SELECT` on `ident.*` and `EXECUTE` on the consent-gated functions:

  - `ident.resolve_repo(...)`, `ident.resolve_actor(...)` (always audit; consent-checked)
  - `ident.repo_public(...)`, `ident.actor_public(...)`
  - Also allowed to call the same upsert shims as API.

- **`broker`** (NOLOGIN) - owner of the **SECURITY DEFINER** consent functions above.
  It has only the minimum required table perms for those bodies to run (read the maps, write audit, read `consent_receipts`). No one logs in as `broker`. This keeps definer power separate from app users.

- **superuser** (e.g., `swearjarbot`) - for init/migrations/Adminer only. Don't run services as this.

### Guiding principles

- **HID-first.** Core tables and queues use HIDs; numeric GH IDs live behind `ident.*`.
- **Consent-gated exposure.** No PII unless there's an active `opt_in`. Resolvers always audit.
- **Schema = scope.** `public` is app data. `ident` is sensitive linkage + audit.
- **Least privilege.**
  - API can't `SELECT` from `ident` at all.
  - Hallmonitor can read `ident` and call resolvers, but not mutate maps directly (except via shims).
  - SECDEF functions are owned by a NOLOGIN role (`broker`) to avoid lateral privilege.
- **Avoid read leaks in writes.** On PG18, `INSERT ... ON CONFLICT` can require read rights. We wrap map writes in tiny SECDEF shims so API/HM can write without gaining read.

### DSNs (env)

_Note: These aren't entirely clean and need to be renamed._

SERVICE_PGSQL_DBURL = postgres://sw_api:sw_api@sw_pgsql/swearjar
SERVICE_PGSQL_DBURL_HM = postgres://sw_hallmonitor:sw_hallmonitor@sw_pgsql/swearjar
SERVICE_PGSQL_DBURL_SU = postgres://swearjarbot:swearjar@sw_pgsql/swearjar # admin only

### Notes

- **Functions pin `search_path`** (`SET search_path = ident, public`) and are **SECURITY DEFINER**.
- **Default privileges** are set so new `public` objects are usable by both app roles; `ident` stays locked down by default.
- If you add new `ident` functions, **own them by `broker`** and grant `EXECUTE` only to `sw_hallmonitor` (and/or the specific caller you intend).
- If you add new `ident` tables, prefer **write via shims** if there's any chance `ON CONFLICT`/UPSERT semantics will be used by API/HM.

That's the model: API writes data, HM resolves/reads when consent allows, `broker` owns the sharp knives.

## API User Smoke Tests

```sql

-- sanity
SELECT current_user, session_user; -- expect sw_api

-- fixed, valid 32-byte HIDs (64 hex chars)
-- repo = 64 'a'; actor = 64 'b'
-- You can copy these verbatim.
-- repo_hid:  aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
-- actor_hid: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb

-- create principals in public
INSERT INTO principals_repos (repo_hid)
VALUES (decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes)
ON CONFLICT DO NOTHING;

INSERT INTO principals_actors (actor_hid)
VALUES (decode('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','hex')::public.hid_bytes)
ON CONFLICT DO NOTHING;

-- write ident maps via SECDEF shims (this is the important part)
SELECT ident.upsert_gh_repo_map(
  decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes,
  123456
);

SELECT ident.upsert_gh_actor_map(
  decode('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','hex')::public.hid_bytes,
  987654
);

-- prove we cannot read/write ident directly (both should ERROR: permission denied)
SELECT count(*) FROM ident.gh_repo_map;
INSERT INTO ident.gh_repo_map (repo_hid, gh_repo_id)
VALUES (decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes, 111);

-- insert a minimal utterance and a hit in public (should succeed)
WITH ins AS (
  INSERT INTO utterances(
    event_id, event_type, repo_hid, actor_hid, hid_key_version, created_at,
    source, source_detail, ordinal, text_raw
  )
  VALUES (
    'evt-smoke-1', 'test',
    decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes,
    decode('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','hex')::public.hid_bytes,
    1, now(),
    'commit', '', 0, 'hello world'
  )
  RETURNING id, repo_hid, actor_hid
)
INSERT INTO hits(
  utterance_id, created_at, source, repo_hid, actor_hid,
  lang_code, term, category, severity, span_start, span_end, detector_version
)
SELECT id, now(), 'commit', repo_hid, actor_hid, 'en', 'foo', 'generic', 'mild', 0, 3, 1 FROM ins;

-- resolvers are forbidden to API (should ERROR: permission denied on function)
SELECT ident.resolve_repo(decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes, 'api','smoke');

```

## HM User Smoke Tests

```sql

-- sanity
SELECT current_user, session_user;  -- expect sw_hallmonitor

-- can read ident maps
SELECT gh_repo_id
FROM ident.gh_repo_map
WHERE repo_hid = decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes;

-- resolver BEFORE consent - should return NULL and write a deny audit row
SELECT ident.resolve_repo(
  decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes,
  'hallmonitor','smoke_denied'
) AS gh_repo_id;

SELECT principal, allowed, reason, who, purpose
FROM ident.identity_resolve_audit
ORDER BY ts DESC LIMIT 1; -- expect allowed=false, reason = 'not allowed by consent'

-- grant consent and resolve again - should return 123456 and write allow audit row
INSERT INTO consent_receipts(
  principal, principal_hid, action, scope, evidence_kind, evidence_url, state
)
VALUES(
  'repo',
  decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes,
  'opt_in',
  ARRAY['demask_repo']::consent_scope_enum[],
  'repo_file',
  'https://example.invalid/proof',
  'active'
)
ON CONFLICT (principal, principal_hid, action) DO NOTHING;

SELECT ident.resolve_repo(
  decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes,
  'hallmonitor','smoke_allowed'
) AS gh_repo_id; -- expect 123456

SELECT principal, allowed, reason, who, purpose
FROM ident.identity_resolve_audit
ORDER BY ts DESC LIMIT 1; -- expect allowed=true, reason = 'opt-in'

-- HM is read-only in ident (write should ERROR: permission denied)
DELETE FROM ident.gh_repo_map WHERE repo_hid = decode('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','hex')::public.hid_bytes;
```
