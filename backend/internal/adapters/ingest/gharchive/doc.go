// Package gharchive handles reading GH Archive hourly gzip files line-by-line
//
// Design choices:
// - Stream with bufio.Scanner but with a 32MB cap to reliably handle huge commits.
// - Strict JSON/v2 via jsonx (UTF-8 validated). Malformed lines are skipped.
// - Keep payload as raw JSON until extract-stage to avoid a giant union type
// - Provide a deterministic UUID builder so callers can key utterances without event_id
package gharchive
