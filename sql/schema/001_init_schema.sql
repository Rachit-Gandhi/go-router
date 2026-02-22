-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  user_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username text NOT NULL,
  email text NOT NULL UNIQUE,
  password text NOT NULL,
  balance_credits_int int NOT NULL DEFAULT 0
);

CREATE TABLE api_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  name text NOT NULL,
  key_hash text NOT NULL UNIQUE,
  disabled boolean NOT NULL DEFAULT false,
  deleted boolean NOT NULL DEFAULT false,
  last_used_at timestamptz,
  disabled_at timestamptz,
  deleted_at timestamptz
);

CREATE TABLE models (
  model_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  model_name text NOT NULL,
  slug text NOT NULL UNIQUE,
  company text NOT NULL
);

CREATE TABLE deployers (
  deployer_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  deployer_name text NOT NULL
);

CREATE TABLE deployed_models (
  deployed_model_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  deployer_id uuid NOT NULL REFERENCES deployers(deployer_id) ON DELETE CASCADE,
  model_id uuid NOT NULL REFERENCES models(model_id) ON DELETE CASCADE,
  input_cost int NOT NULL,
  output_cost int NOT NULL
);

CREATE TABLE user_usage (
  apikey_id uuid NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  deployed_model_id uuid NOT NULL REFERENCES deployed_models(deployed_model_id) ON DELETE CASCADE,
  input_tokens int NOT NULL,
  output_tokens int NOT NULL,
  query text NOT NULL,
  resp text NOT NULL,
  credits_used int NOT NULL
);

CREATE TABLE razorpay_transactions (
  transaction_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  amt int NOT NULL,
  credits int NOT NULL,
  status text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

CREATE INDEX api_keys_user_id_idx ON api_keys(user_id);
CREATE INDEX deployed_models_deployer_id_idx ON deployed_models(deployer_id);
CREATE INDEX deployed_models_model_id_idx ON deployed_models(model_id);
CREATE INDEX user_usage_apikey_id_idx ON user_usage(apikey_id);
CREATE INDEX user_usage_user_id_idx ON user_usage(user_id);
CREATE INDEX user_usage_deployed_model_id_idx ON user_usage(deployed_model_id);
CREATE INDEX razorpay_transactions_user_id_idx ON razorpay_transactions(user_id);
CREATE INDEX razorpay_transactions_started_at_idx ON razorpay_transactions(started_at);
CREATE INDEX razorpay_transactions_completed_at_idx ON razorpay_transactions(completed_at);

-- +goose Down
DROP INDEX IF EXISTS razorpay_transactions_completed_at_idx;
DROP INDEX IF EXISTS razorpay_transactions_started_at_idx;
DROP INDEX IF EXISTS razorpay_transactions_user_id_idx;
DROP INDEX IF EXISTS user_usage_deployed_model_id_idx;
DROP INDEX IF EXISTS user_usage_user_id_idx;
DROP INDEX IF EXISTS user_usage_apikey_id_idx;
DROP INDEX IF EXISTS deployed_models_model_id_idx;
DROP INDEX IF EXISTS deployed_models_deployer_id_idx;
DROP INDEX IF EXISTS api_keys_user_id_idx;

DROP TABLE IF EXISTS razorpay_transactions;
DROP TABLE IF EXISTS user_usage;
DROP TABLE IF EXISTS deployed_models;
DROP TABLE IF EXISTS deployers;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
