# Readme is currently a scratchpad

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2025-08-01T00 -end 2025-08-01T02 --detect --detver 1'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-detect -start 2025-08-01T00 -end 2025-08-01T02'

docker exec -it sw_api bash -c 'rm /var/lib/swearjar/gharchive/\*'

docker exec -it sw_api bash -c 'ls /var/lib/swearjar/gharchive'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode backfill --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode worker -concurrency 4 -rps 2 -burst 4'

Backfill ALL)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2011-02-12T00 -end 2025-09-11T00 --detect --detver 1'

Backfill plan only)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2011-02-12T00 -end 2011-03-01T00 --plan-only'

Backfill resume)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill --resume'

# Around when data started breaking) `2012-03-10-00` to `2012-04-04-12-00`

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2012-03-10T00 -end 2025-09-11T00'
docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-detect -start 2012-03-10T00 -end 2025-09-11T00'

# Starting to validate the concept... Win

```sql
WITH
  (SELECT min(created_at) FROM swearjar.hits) AS min_ts,
  (SELECT max(created_at) FROM swearjar.hits) AS max_ts,
  toUnixTimestamp(min_ts) AS min_x,
  toUnixTimestamp(max_ts) AS max_x_raw,
  IF(min_x = max_x_raw, max_x_raw + 1, max_x_raw) AS max_x,
  60 AS buckets
SELECT
  term,
  sum(hits) AS total_hits,
  sparkbar(buckets, min_x, max_x)(toUnixTimestamp(day), hits) AS spark
FROM
  (SELECT term, toDate(created_at) AS day, count() AS hits FROM swearjar.hits GROUP BY term, day)
GROUP BY term
ORDER BY total_hits DESC, term ASC;
```

```
SELECT database, table,
  formatReadableSize(sum(bytes)) AS size,
  formatReadableSize(sum(data_uncompressed_bytes)) AS uncomp
FROM system.parts
WHERE active AND database = 'swearjar'
GROUP BY database, table
ORDER BY sum(bytes) DESC;
```
