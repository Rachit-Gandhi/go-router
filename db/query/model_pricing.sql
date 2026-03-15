-- name: ListActiveModelPricing :many
SELECT id, provider, model, input_price_per_mtok, output_price_per_mtok, currency, source, effective_from, effective_to, created_at
FROM model_pricing
WHERE effective_to IS NULL
ORDER BY provider, model, effective_from DESC;

-- name: GetActiveModelPricingByProviderModel :one
SELECT id, provider, model, input_price_per_mtok, output_price_per_mtok, currency, source, effective_from, effective_to, created_at
FROM model_pricing
WHERE provider = $1
  AND model = $2
  AND effective_to IS NULL
ORDER BY effective_from DESC
LIMIT 1;

-- name: CloseActiveModelPricing :execrows
UPDATE model_pricing
SET effective_to = $3
WHERE provider = $1
  AND model = $2
  AND effective_to IS NULL
  AND effective_from < $3;

-- name: CreateModelPricing :one
INSERT INTO model_pricing (
  provider,
  model,
  input_price_per_mtok,
  output_price_per_mtok,
  currency,
  source,
  effective_from
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, provider, model, input_price_per_mtok, output_price_per_mtok, currency, source, effective_from, effective_to, created_at;
