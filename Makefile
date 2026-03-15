SQLC_CMD = go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
GOOSE_CMD = go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
MIGRATIONS_DIR = db/migrations

.PHONY: test sqlc-generate migrate-up migrate-down

test:
	go test ./...

sqlc-generate:
	$(SQLC_CMD) generate -f db/sqlc.yaml

migrate-up:
	@if [ -z "$(DB_DSN)" ]; then echo "DB_DSN is required"; exit 1; fi
	$(GOOSE_CMD) -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

migrate-down:
	@if [ -z "$(DB_DSN)" ]; then echo "DB_DSN is required"; exit 1; fi
	$(GOOSE_CMD) -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down
