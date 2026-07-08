package plugin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/plugin/builtin/auth"
	"github.com/mahin273/RateMesh/internal/plugin/builtin/transform"
)

func TestJWTAuthPlugin(t *testing.T) {
	jwtPlugin := auth.NewJWTAuthPlugin()
	secret := "test-secret-key"

	// Create a valid test JWT
	token := jwt.New(jwt.SigningMethodHS256)
	tokenStr, _ := token.SignedString([]byte(secret))

	tests := []struct {
		name           string
		configJSON     string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "missing config fails closed",
			configJSON:     "",
			authHeader:     "Bearer " + tokenStr,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing authorization header",
			configJSON:     `{"secret": "test-secret-key"}`,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token signature",
			configJSON:     `{"secret": "wrong-secret-key"}`,
			authHeader:     "Bearer " + tokenStr,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token allowed",
			configJSON:     `{"secret": "test-secret-key"}`,
			authHeader:     "Bearer " + tokenStr,
			expectedStatus: 0, // allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			ctx := context.Background()
			if tt.configJSON != "" {
				configMap := map[string][]byte{
					"auth": []byte(tt.configJSON),
				}
				ctx = plugin.AttachPluginConfigs(ctx, configMap)
			}

			rc := &plugin.RequestContext{
				TenantID: "tenant-1",
				Route:    "/*",
				Request:  req,
			}

			sc, err := jwtPlugin.OnRequest(ctx, rc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectedStatus == 0 {
				if sc != nil {
					t.Errorf("expected request to be allowed, got short-circuit status %d", sc.StatusCode)
				}
			} else {
				if sc == nil {
					t.Fatal("expected request to be short-circuited, got nil")
				}
				if sc.StatusCode != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, sc.StatusCode)
				}
			}
		})
	}
}

func TestTransformPlugin(t *testing.T) {
	transPlugin := transform.NewTransformPlugin()
	configJSON := `{
		"request": {
			"add": {"X-Added-Req": "yes"},
			"remove": ["X-Remove-Req"]
		},
		"response": {
			"add": {"X-Added-Res": "sure"},
			"remove": ["X-Remove-Res"]
		}
	}`

	configMap := map[string][]byte{
		"transform": []byte(configJSON),
	}
	ctx := plugin.AttachPluginConfigs(context.Background(), configMap)

	t.Run("OnRequest Header Transformation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Remove-Req", "temp-val")

		rc := &plugin.RequestContext{
			TenantID: "tenant-1",
			Route:    "/*",
			Request:  req,
		}

		sc, err := transPlugin.OnRequest(ctx, rc)
		if err != nil || sc != nil {
			t.Fatalf("unexpected request intercept: %v, %v", err, sc)
		}

		if req.Header.Get("X-Added-Req") != "yes" {
			t.Errorf("expected X-Added-Req to be 'yes', got %q", req.Header.Get("X-Added-Req"))
		}
		if req.Header.Get("X-Remove-Req") != "" {
			t.Error("expected X-Remove-Req to be removed")
		}
	})

	t.Run("OnResponse Header Transformation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		resp := rec.Result()
		resp.Header.Set("X-Remove-Res", "remove-val")

		rc := &plugin.ResponseContext{
			TenantID: "tenant-1",
			Response: resp,
		}

		err := transPlugin.OnResponse(ctx, rc)
		if err != nil {
			t.Fatalf("unexpected response intercept error: %v", err)
		}

		if resp.Header.Get("X-Added-Res") != "sure" {
			t.Errorf("expected X-Added-Res to be 'sure', got %q", resp.Header.Get("X-Added-Res"))
		}
		if resp.Header.Get("X-Remove-Res") != "" {
			t.Error("expected X-Remove-Res to be removed")
		}
	})
}
