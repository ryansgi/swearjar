# Rulepack v2 - Composable Schema README

This README explains the **composable** Rulepack v2 format used by Github swearjar: what's in the core, how per‑language fragments work, how composition/merging happens, and how to extend it safely.

---

## TL;DR

- Put global stuff in `rules/1/core.json`.
- Put language‑specific stuff in `rules/1/<lang>/` split by concern: `*.core.json`, `*.lemmas.json`, `*.bot_rage.json`, `*.tool_rage.json`, `*.lang_rage.json`, `*.generic.json`.
- Validate `core.json` with `schema/pack.core.schema.json` and fragments with `schema/pack.fragment.schema.json`.
- Loader walks the tree, validates, **merges** (append + de‑dupe), expands **slots** (`{TARGET_*}`) and **variants** (leet/gapped/etc), then compiles regex + the lemma automaton.

---

## Directory layout

```
rules/
└─ 1/
   ├─ core.json                     # global config & slots
   ├─ schema/
   │  ├─ pack.core.schema.json      # JSON Schema for core.json
   │  └─ pack.fragment.schema.json  # JSON Schema for language fragments
   ├─ en/
   │  ├─ en.core.json
   │  ├─ en.lemmas.json
   │  ├─ en.bot_rage.json
   │  ├─ en.tool_rage.json
   │  ├─ en.lang_rage.json
   │  └─ en.generic.json
   ├─ ja/
   │  └─ ...
   └─ ar/
      └─ ...
```

---

## Concepts

### Categories

We use the following categories in v2:

- `generic` - frustration and general profanity (i.e. crap, wtf, trash).
- `harassment` - insults that target a subject (non‑slur here).
- `self_own` - self‑blame (i.e. "we messed up").
- `tooling_rage` - rage directed at tools/pipelines/linters.
- `bot_rage` - rage directed at bots/services.
- `lang_rage` - rage at languages/frameworks (i.e. "javascript is trash").

### Zones

Engines may tag text spans as `plain`, `quote`, `code`, `identifier`, `url`, `email`, `emoji`. Severity can be boosted/dampened by zone (see `severity_mods`).

### Variants

Variant handlers produce "shadow strings" for matching:

- `leet` - conservative leetspeak mapping.
- `gapped` - allow punctuation/spaces between letters (i.e. f.u.c.k).
- `repeat_collapse` - collapse long character runs (fuuuuuck → fuuck/fuck).
- `confusables` - map Unicode confusables to ASCII skeleton.

### Slots (targets)

Slots let templates stay compact:

- `{TARGET_BOT}`, `{TARGET_TOOL}`, `{TARGET_LANG}`, `{TARGET_FRAMEWORK}`.
  Define aliases in **core**; language fragments **use** them but don't redefine.

### Lemmas vs Templates

- **Lemmas**: fast multi‑substring rules (i.e. "trash", "merde", "ゴミ").
- **Templates**: regex patterns for structured phrases and targeted rage.

---

## Core file (`core.json`)

Holds shared knobs:

- `version`: 2
- `meta`: name, generated_at
- `categories`: list above
- `variants_spec`: descriptions (engine hints only)
- `zones`: names + notes
- `slots`: alias lists for `{TARGET_*}`
- `allowlist`: global & zone‑specific whitelists to avoid false positives
- `engine_hints`: normalization + search strategy
- `severity_mods`: context‑based boosts/dampening

Core is validated by `schema/pack.core.schema.json`.

---

## Language fragments (`<lang>/*.json`)

Each fragment is validated by `schema/pack.fragment.schema.json` and can include:

- `language`: ISO code (i.e. `en`, `ja`).
- `lemmas`: array of `{ term, category, severity, variants?, context_signals? }`.
- `templates`: array of `{ id, pattern, category, severity, variants?, context_signals?, examples? }`.
- `allowlist`: language add‑ons, usually zone‑scoped (i.e. common code words in Japanese/Arabic).
- `engine_hints`: language normalization tweaks (i.e. Arabic diacritics, Japanese NFKC + カタカナ → ひらがな).

**Naming conventions** for files:

- `<lang>.core.json` - optional per‑language allowlists or engine hints.
- `<lang>.lemmas.json` - lemma tokens only.
- `<lang>.<category>.json` - templates for one category (i.e. `en.bot_rage.json`).

**ID conventions** for templates:

- `lang.category.slug`, i.e. `en.tool_rage.keeps_breaking`.
  IDs must be unique across the whole pack.

---

## Composition & merge rules

The loader composes **core + all fragments** into a single in‑memory pack:

1. **Load & validate** `core.json` against `pack.core.schema.json`.
2. Recursively **discover fragments** under `rules/1/**/<lang>/*.json`.
3. For each fragment, **validate** against `pack.fragment.schema.json`.
4. **Merge** fields into the pack (order independent):

   - `lemmas`: append, then **de‑dupe** by `(language, lowercased term)`.
   - `templates`: append; enforce **unique `id`**; warn on duplicate `id`.
   - `allowlist`: **deep‑merge** (language entries extend/override core).
   - `engine_hints`: **deep‑merge** (per‑language overrides add to core).

5. **Expand slots** inside `pattern`: replace `{TARGET_*}` with a non‑capturing group of **escaped** names from core slots.
6. Apply **variant expansions** as "shadow strings" for leet/gapped/confusables.
7. **Compile templates** (regex) and **build the lemma automaton**.

> Tip: Annotate each compiled regex with its language and category for telemetry and tuning.

---

## Normalization (must‑have)

Always normalize before scanning; recommended settings (already in core):

- Casefold + NFKC.
- Strip variation selectors & zero‑width marks.
- Collapse whitespace & repeated chars (threshold=3).
- Confusables skeleton mapping (ASCII‑preferring).
- Per‑language tweaks:

  - **Arabic**: remove diacritics/kashida; unify Hamza, Alef, Ya/Alif Maqsura; تاء مربوطة→ه.
  - **Japanese**: NFKC, Katakana→Hiragana, normalize dakuten/handakuten, collapse ー.
  - **Korean**: normalize compatibility Jamo.

---

## Severity model

Use `severity` 0-3 on rules:

- 1: mild profanity/frustration (crap, wtf, rubbish)
- 2: stronger profanity/insults, tool/bot/lang rage
- 3: highest toxicity (i.e. "die/死ね" at a target)

`severity_mods` can boost/dampen:

- `boost.direct_address` (+1) when directly addressing a target (`fuck you, <bot>`)
- `boost.mention_target` (+1) when a slot alias is @mentioned
- `boost.repetition` (+1) for repeated tokens
- `reduce.in_code/identifier/quote` (−1) where false positives are common

---

## Writing good templates

**Do**

- Scope by category (`tooling_rage`, `bot_rage`, `lang_rage`).
- Prefer explicit verbs/adjectives, i.e. `keeps breaking`, `is trash`.
- Use slots: `{TARGET_TOOL}`, `{TARGET_BOT}`, `{TARGET_LANG}`/`{TARGET_FRAMEWORK}`.
- Add `variants` if the phrase is leet/gapped‑prone.
- Add `context_signals` for precision, i.e. `{ "disallow_in_code": true }`.

**Avoid**

- `\b` in CJK/Arabic; rely on script/zone boundaries or simple run segmentation.
- Over‑broad wildcards like `.*` unless bounded by language/script.

**Example (English tool rage)**

```json
{
  "id": "en.tool_rage.keeps_breaking",
  "pattern": "\\b(?:{TARGET_TOOL})\\s+(?:keeps|kept|is|was|continues\\s*to)\\s+(?:fail|failing|break(?:ing)?|borked|trash|garbage|useless|flaky)\\b",
  "category": "tooling_rage",
  "severity": 2,
  "variants": ["repeat_collapse", "confusables"],
  "context_signals": { "disallow_in_code": true }
}
```

**Example (Japanese bot rage)**

```json
{
  "id": "ja.bot_rage.shine",
  "pattern": "(?:{TARGET_BOT})\\s*(?:は)?\\s*死ね",
  "category": "bot_rage",
  "severity": 3,
  "variants": ["repeat_collapse", "confusables"]
}
```

---

## Lemma hygiene

- Keep lemmas short, common, and **non‑sexual** unless policy requires.
- Add `variants` for leet/repeats when those forms are common.
- Use the **allowlist** to suppress false positives (i.e. `sass` in code).

---

## Extending the pack

### Add a new language `xx`

1. Create `rules/1/xx/`.
2. (Optional) `xx.core.json` for allowlist/engine hints.
3. Add `xx.lemmas.json` with a few core terms.
4. Add one or more category files: `xx.tool_rage.json`, `xx.bot_rage.json`, `xx.lang_rage.json`.
5. Validate fragments against `schema/pack.fragment.schema.json`.
6. Run the loader; it composes everything automatically.

### Add a new category (example: `build_rage`)

1. Add the category to `core.json > categories`.
2. Create per‑language fragments like `en.build_rage.json`.
3. Write templates using the existing slots or add a new slot to `core.json` if needed.

### Extend English lemmas from a list

1. Curate terms (exclude sexual terms/slurs unless policy requires).
2. Append to `en/en.lemmas.json` with conservative severities.
3. Prefer phrases → templates if boundaries matter.

---

## Composition & validation in CI

- **Validate** core and fragments with the JSON Schemas (i.e. `ajv`, `jsonschema`, `djv`).
- **Lint**: ensure template `id` uniqueness and check slot names exist.
- **Test**: keep golden test files with "should match / should not match".
- **Telemetry**: log rule IDs and spans for real‑world tuning.

---

## Loader outline (pseudocode)

```bash
docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-rulepacker'
```

---

## Gotchas & tips

- Avoid `\b` in CJK/Arabic; prefer script run boundaries + zones.
- Keep **IDs stable**; they're how we audit and tune.
- Use `context_signals.disallow_in_code` to reduce false positives from log snippets and stack traces.
- Don't duplicate slot aliases across languages—put them in core once.
- Keep fragments small and focused; prefer **many small files** over one giant blob.

---

## FAQ

**Q: Why separate lemmas and templates?** Faster scanning (lemmas) + precise phrasing (templates). Different engines can optimize each path.

**Q: How do I add a new bot/tool?** Edit `core.json > slots > TARGET_BOT/TARGET_TOOL` and let slot expansion handle all templates automatically.

**Q: Can I override severities per language?** Yes—set severity in the fragment. Core only defines categories and modifiers.

**Q: Where do I tune normalization?** Put global defaults in `core.json > engine_hints.normalization`; override per language in `<lang>.core.json`.

---

## Changelog policy

Bump `version` only for breaking schema changes. Content changes (new tokens/templates) are _data_ changes and tracked via Git history.
