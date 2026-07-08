package policy

import "context"

type contextKey string

const TenantKey contextKey = "tenant"

// AttachTenantToContext attaches the tenant object to the request context.
func AttachTenantToContext(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, TenantKey, tenant)
}

// GetTenantFromContext retrieves the tenant object from the context.
func GetTenantFromContext(ctx context.Context) *Tenant {
	if val := ctx.Value(TenantKey); val != nil {
		if t, ok := val.(*Tenant); ok {
			return t
		}
	}
	return nil
}
