-- +goose Up
CREATE TYPE rate_limit_strategy AS ENUM ('token_bucket', 'sliding_window');
CREATE TYPE rate_limit_mode AS ENUM ('strict', 'eventual');

CREATE TABLE route_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    route_pattern TEXT NOT NULL,
    method TEXT NOT NULL,
    strategy rate_limit_strategy NOT NULL DEFAULT 'token_bucket',
    mode rate_limit_mode NOT NULL DEFAULT 'eventual',
    limit_per_window INT NOT NULL,
    window_seconds INT NOT NULL,
    burst_allowance INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE route_policies;
DROP TYPE rate_limit_mode;
DROP TYPE rate_limit_strategy;
