# Rulepack Authoring Guide

The **rulepack** defines profanity detection rules for Swearjar. It is embedded as JSON (`rules.json`) and compiled into regex templates and lemma sets at runtime.

---

## Structure of `rules.json`

```json
{
  "version": 1,
  "templates": [
    { "pattern": "fuck you {TARGET_BOT}", "category": "tool", "severity": 3 },
    { "pattern": "screw you {TARGET_TOOL}", "category": "tool", "severity": 2 }
  ],
  "slots": {
    "TARGET_BOT": ["dependabot", "renovate", "mergify"],
    "TARGET_TOOL": ["lint", "eslint", "ci"]
  },
  "lemmas": [
    { "term": "fuck", "category": "generic", "severity": 2 },
    { "term": "shit", "category": "generic", "severity": 1 }
  ],
  "stoplist": ["assistant", "assess", "Scunthorpe"]
}
```

---

## Sections

### `version`

Bumped whenever templates, slots, or lemmas change. Stored in hits (`detector_version`) for backfill compatibility.

### `templates`

Regex-like patterns with optional slot placeholders:

- `{SLOT}` expands to an alternation group of terms from `slots`.
- Run **before lemmas**.
- Example: `"fuck you {TARGET_BOT}"` -> `fuck you (?:dependabot|renovate|mergify)`

### `slots`

Reusable term lists for templates. Expansion is literal-escaped.

- Use for tools, bots, services, etc.
- Example: `TARGET_TOOL` = \["lint", "eslint", "ci"]

### `lemmas`

Lowercase word stems for multi-pattern search.

- Backstop for general profanity not caught by templates.
- Must be **single terms** (no spaces).
- Example: `fuck`, `shit`, `bitch`

### `stoplist`

Guard words to avoid false positives.

- If lemma occurs inside these, it is ignored.
- Example: `Scunthorpe` prevents matching `cunt` inside the town name.

---

## Severity Levels

- **1 (mild):** lightweight, common slang (e.g. `shit`).
- **2 (moderate):** stronger insults (e.g. `fuck`, `bitch`).
- **3 (directed):** insults aimed at bots/tools via templates (e.g. `fuck you dependabot`).

These are arbitrary but consistent; downstream analytics/UI uses them.

---

## Authoring Guidelines

1. **Prefer templates first**

   - Capture context-specific profanity ("fuck you {BOT}") before generic lemmas.

2. **Keep slots tight**

   - Only list real bots/tools. Avoid wildcards that may overmatch.

3. **Avoid overreach**

   - Don't add short lemmas like `ass` without careful stoplist coverage.

4. **Stoplist aggressively**

   - Use for words with benign overlaps (`assistant`, `passage`).

5. **Version bump**

   - Any change to `rules.json` -> increment `version`.
   - Ensures new hits are distinguished and backfills can run.

6. **Test each addition**

   - Add positive + negative test cases in `pack_test.go` or detector tests.

---

## Workflow for Adding Rules

1. Edit `internal/rulepack/rules.json`.
2. Run tests in `internal/rulepack` and `internal/detector`.
3. Bump `version`.
4. Commit with message: `rulepack: bump to vN, add [rule summary]`.

---

## Future Directions

- Multi-language packs (`rules_en.json`, `rules_fr.json` etc.).
- Richer categories (slur vs tool vs self-directed).
- Admin console for editing & regenerating rulepacks.
- Swappable storage of rulepacks (DB vs embed).
