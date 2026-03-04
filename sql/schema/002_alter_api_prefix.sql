-- +goose Up
ALTER TABLE api_keys ADD COLUMN api_key_show_string text NOT NULL DEFAULT 'go-';

-- +goose Down
ALTER TABLE api_keys DROP COLUMN api_key_show_string;