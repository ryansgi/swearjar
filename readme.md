# Readme is currently a scratchpad

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2025-08-01T00 -end 2025-08-01T02 --detect --detver 1'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-detect -start 2025-08-01T00 -end 2025-08-01T02'

docker exec -it sw_api bash -c 'rm /var/lib/swearjar/gharchive/\*'

docker exec -it sw_api bash -c 'ls /var/lib/swearjar/gharchive'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode backfill --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode worker -concurrency 4 -rps 2 -burst 4'

```
SELECT database, table,
  formatReadableSize(sum(bytes)) AS size,
  formatReadableSize(sum(data_uncompressed_bytes)) AS uncomp
FROM system.parts
WHERE active AND database = 'swearjar'
GROUP BY database, table
ORDER BY sum(bytes) DESC;
```

Backfill ALL)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2011-02-12T00 -end 2025-09-11T00 --detect --detver 1'

Backfill plan only)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2011-02-12T00 -end 2011-03-01T00 --plan-only'

Backfill resume)

- docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill --resume'
