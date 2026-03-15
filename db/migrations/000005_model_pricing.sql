-- +goose Up
CREATE TABLE IF NOT EXISTS model_pricing (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    input_price_per_mtok DOUBLE PRECISION NOT NULL CHECK (input_price_per_mtok >= 0),
    output_price_per_mtok DOUBLE PRECISION NOT NULL CHECK (output_price_per_mtok >= 0),
    currency TEXT NOT NULL DEFAULT 'USD',
    source TEXT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_pricing_provider_model_effective_from
    ON model_pricing (provider, model, effective_from);

CREATE INDEX IF NOT EXISTS idx_model_pricing_active_lookup
    ON model_pricing (provider, model, effective_from DESC)
    WHERE effective_to IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_model_pricing_active_lookup;
DROP INDEX IF EXISTS idx_model_pricing_provider_model_effective_from;
DROP TABLE IF EXISTS model_pricing;
