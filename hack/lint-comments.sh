#!/usr/bin/env bash
set -euo pipefail

# --- Collect target files -----------------------------------------------------
files=("$@")
if [ ${#files[@]} -eq 0 ]; then
  mapfile -t files < <(git ls-files '*.go')
fi
[ ${#files[@]} -eq 0 ] && exit 0

fail=0
for f in "${files[@]}"; do
  [[ "$f" == vendor/* ]] && continue
  if ! awk -f - "$f" <<'AWK'
function is_comment_line(line)      { return match(line, /^[[:space:]]*\/\//) }
function comment_text(line)         { sub(/^[[:space:]]*\/\/[[:space:]]?/, "", line); return line }
function skip_line(txt) {
  if (txt ~ /^[[:space:]]*go:/)               return 1
  if (txt ~ /^[[:space:]]*nolint\b/)          return 1
  if (txt ~ /^[[:space:]]*Code generated\b/)  return 1
  if (txt ~ /^[[:space:]]*Package\b/)         return 1
  if (txt ~ /^[[:space:]]*https?:\/\//)       return 1
  return 0
}
function ends_with_bad_period(txt) {
  if (txt ~ /\.[[:space:]]*$/) {
    if (txt ~ /\.{3}[[:space:]]*$/)                          return 0  # ...
    if (txt ~ /(^|[^A-Za-z])(e\.g|i\.e|etc)\.[[:space:]]*$/) return 0  # e.g. i.e. etc.
    return 1
  }
  return 0
}
function first_unsafe(txt,   rstart, rlen) {
  if (match(txt, unsafe)) return substr(txt, RSTART, RLENGTH)
  return ""
}
function report(file, line, msg) { printf("%s:%d: %s\n", file, line, msg) }

BEGIN {
  # Unsafe = any char NOT in whitelist.
  # Converted \s -> [:space:] and escaped for ERE.
  unsafe = "[^a-zA-Z0-9[:space:]\\/\\*\\+<\\^>;:{}()\\[\\]=\\.,'\"`~&?_\\-%\\$@\\|!#-]"
}

{
  file = FILENAME; line = FNR; text = $0

  if (is_comment_line(text)) {
    txt = comment_text(text)

    if (!skip_line(txt)) {
      bad = first_unsafe(txt)
      if (bad != "") {
        # avoid literal single quotes in this string by using %c (39)
        report(file, line, sprintf("comment contains unsafe character %c%s%c", 39, bad, 39))
        violations++
      }
    }

    inblock   = 1
    last_line = line
    last_txt  = txt
    last_keep = !skip_line(txt)
  } else {
    if (inblock) {
      if (last_keep && ends_with_bad_period(last_txt)) {
        report(file, last_line, "final line of comment block should not end with a period")
        violations++
      }
      inblock = 0
    }
  }
}

END {
  if (inblock && last_keep && ends_with_bad_period(last_txt)) {
    report(FILENAME, last_line, "final line of comment block should not end with a period")
    violations++
  }
  if (violations > 0) exit 1
}
AWK
  then
    fail=1
  fi
done

exit $fail
