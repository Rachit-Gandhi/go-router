-- +goose Up
-- Partition maintenance: pre-create current and next month partitions for usage_logs.
DO $$
DECLARE
    current_month DATE := date_trunc('month', NOW())::date;
    next_month DATE := (date_trunc('month', NOW()) + INTERVAL '1 month')::date;
    month_after_next DATE := (date_trunc('month', NOW()) + INTERVAL '2 month')::date;
    partition_name TEXT;
BEGIN
    partition_name := format('usage_logs_%s', to_char(current_month, 'YYYYMM'));
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF usage_logs FOR VALUES FROM (%L) TO (%L);',
        partition_name, current_month, next_month
    );

    partition_name := format('usage_logs_%s', to_char(next_month, 'YYYYMM'));
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF usage_logs FOR VALUES FROM (%L) TO (%L);',
        partition_name, next_month, month_after_next
    );
END
$$;

-- +goose Down
DO $$
DECLARE
    current_month DATE := date_trunc('month', NOW())::date;
    next_month DATE := (date_trunc('month', NOW()) + INTERVAL '1 month')::date;
    partition_name TEXT;
BEGIN
    partition_name := format('usage_logs_%s', to_char(next_month, 'YYYYMM'));
    EXECUTE format('DROP TABLE IF EXISTS %I;', partition_name);

    partition_name := format('usage_logs_%s', to_char(current_month, 'YYYYMM'));
    EXECUTE format('DROP TABLE IF EXISTS %I;', partition_name);
END
$$;
