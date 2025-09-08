# Readme is currently a scratchpad

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2025-08-01T00 -end 2025-08-01T02 --detect --detver 1'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-detect -start 2025-08-01T00 -end 2025-08-01T02'

docker exec -it sw_api bash -c 'rm /var/lib/swearjar/gharchive/\*'

docker exec -it sw_api bash -c 'ls /var/lib/swearjar/gharchive'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode backfill --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor --since 2025-08-01T00 --until 2025-09-01T00 --limit 0'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-hallmonitor -mode worker -concurrency 4 -rps 2 -burst 4'
