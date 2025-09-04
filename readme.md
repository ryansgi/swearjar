docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-backfill -start 2025-08-01T00 -end 2025-08-01T02'

docker exec -it sw_api bash -c 'GOEXPERIMENT=jsonv2 go run ./cmd/swearjar-detect -start 2025-08-01T00 -end 2025-08-01T02'
