-- +goose Up
CREATE INDEX IF NOT EXISTS idx_health_checks_service ON health_checks (service);
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at_desc ON health_checks (checked_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_health_checks_checked_at_desc;
DROP INDEX IF EXISTS idx_health_checks_service;
