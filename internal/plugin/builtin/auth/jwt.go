package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mahin273/RateMesh/internal/plugin"
)

type JWTAuthPlugin struct{}

// JWTConfig holds the secret key for validating tokens.
type JWTConfig struct {
	Secret string `json:"secret"`
}

// NewJWTAuthPlugin creates a new instance of the JWTAuthPlugin.
func NewJWTAuthPlugin() plugin.GatewayPlugin {
	return &JWTAuthPlugin{}
}

func (p *JWTAuthPlugin) Name() string {
	return "auth"
}

func (p *JWTAuthPlugin) Priority() int {
	return 10 // Auth runs early in the request chain
}

func (p *JWTAuthPlugin) OnRequest(ctx context.Context, rc *plugin.RequestContext) (*plugin.ShortCircuit, error) {
	configBytes := plugin.GetPluginConfig(ctx, p.Name())
	if configBytes == nil {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("JWT configuration missing"),
		}, nil
	}

	var cfg JWTConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil || cfg.Secret == "" {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("Invalid JWT configuration"),
		}, nil
	}

	authHeader := rc.Request.Header.Get("Authorization")
	if authHeader == "" {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("Missing Authorization header"),
		}, nil
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("Invalid Authorization header format"),
		}, nil
	}

	tokenStr := parts[1]
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil || !token.Valid {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("Invalid or expired token"),
		}, nil
	}

	return nil, nil
}

func (p *JWTAuthPlugin) OnResponse(ctx context.Context, rc *plugin.ResponseContext) error {
	return nil
}
