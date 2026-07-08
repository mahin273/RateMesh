-- +goose Up
INSERT INTO tenants (id, name, plan_tier)
VALUES ('d04a6cb1-5bb5-4c07-b08e-327c62d08a54', 'Load Test Tenant', 'enterprise')
ON CONFLICT (id) DO NOTHING;

INSERT INTO route_policies (id, tenant_id, route_pattern, method, strategy, mode, limit_per_window, window_seconds, burst_allowance, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'd04a6cb1-5bb5-4c07-b08e-327c62d08a54',
    '/api/v1/strict',
    'GET',
    'token_bucket',
    'strict',
    1000,
    10,
    100,
    now(),
    now()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO route_policies (id, tenant_id, route_pattern, method, strategy, mode, limit_per_window, window_seconds, burst_allowance, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'd04a6cb1-5bb5-4c07-b08e-327c62d08a54',
    '/api/v1/eventual',
    'GET',
    'token_bucket',
    'eventual',
    2000,
    10,
    200,
    now(),
    now()
) ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM route_policies WHERE tenant_id = 'd04a6cb1-5bb5-4c07-b08e-327c62d08a54';
DELETE FROM tenants WHERE id = 'd04a6cb1-5bb5-4c07-b08e-327c62d08a54';
