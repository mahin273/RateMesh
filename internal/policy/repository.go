package policy

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository interface {
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	GetRoutePoliciesByTenant(ctx context.Context, tenantID string) ([]*RoutePolicy, error)
	CreateTenant(ctx context.Context, tenant *Tenant) error
	CreateRoutePolicy(ctx context.Context, policy *RoutePolicy) error
	GetPluginsByTenant(ctx context.Context, tenantID string) ([]*Plugin, error)
	CreatePlugin(ctx context.Context, plugin *Plugin) error
}

type sqlRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

func (r *sqlRepository) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	query := `SELECT id, name, plan_tier, created_at FROM tenants WHERE id = $1`
	var t Tenant
	err := r.db.QueryRowContext(ctx, query, id).Scan(&t.ID, &t.Name, &t.PlanTier, &t.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &t, nil
}

func (r *sqlRepository) GetRoutePoliciesByTenant(ctx context.Context, tenantID string) ([]*RoutePolicy, error) {
	query := `
		SELECT id, tenant_id, route_pattern, method, strategy, mode, limit_per_window, window_seconds, burst_allowance, created_at, updated_at
		FROM route_policies
		WHERE tenant_id = $1
		ORDER BY length(route_pattern) DESC` // Specific routes first

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query route policies: %w", err)
	}
	defer rows.Close()

	var policies []*RoutePolicy
	for rows.Next() {
		var p RoutePolicy
		err := rows.Scan(
			&p.ID, &p.TenantID, &p.RoutePattern, &p.Method, &p.Strategy, &p.Mode,
			&p.LimitPerWindow, &p.WindowSeconds, &p.BurstAllowance, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan route policy: %w", err)
		}
		policies = append(policies, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return policies, nil
}

func (r *sqlRepository) CreateTenant(ctx context.Context, tenant *Tenant) error {
	query := `INSERT INTO tenants (id, name, plan_tier, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, tenant.ID, tenant.Name, tenant.PlanTier, tenant.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}
	return nil
}

func (r *sqlRepository) CreateRoutePolicy(ctx context.Context, policy *RoutePolicy) error {
	query := `
		INSERT INTO route_policies (id, tenant_id, route_pattern, method, strategy, mode, limit_per_window, window_seconds, burst_allowance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.db.ExecContext(
		ctx, query, policy.ID, policy.TenantID, policy.RoutePattern, policy.Method,
		policy.Strategy, policy.Mode, policy.LimitPerWindow, policy.WindowSeconds,
		policy.BurstAllowance, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create route policy: %w", err)
	}
	return nil
}

func (r *sqlRepository) GetPluginsByTenant(ctx context.Context, tenantID string) ([]*Plugin, error) {
	query := `
		SELECT id, tenant_id, plugin_name, config_json, enabled, priority
		FROM plugins
		WHERE tenant_id = $1 AND enabled = true
		ORDER BY priority ASC`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query plugins: %w", err)
	}
	defer rows.Close()

	var plugins []*Plugin
	for rows.Next() {
		var p Plugin
		err := rows.Scan(&p.ID, &p.TenantID, &p.PluginName, &p.ConfigJSON, &p.Enabled, &p.Priority)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plugin: %w", err)
		}
		plugins = append(plugins, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return plugins, nil
}

func (r *sqlRepository) CreatePlugin(ctx context.Context, p *Plugin) error {
	query := `
		INSERT INTO plugins (id, tenant_id, plugin_name, config_json, enabled, priority)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, p.ID, p.TenantID, p.PluginName, p.ConfigJSON, p.Enabled, p.Priority)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}
	return nil
}
