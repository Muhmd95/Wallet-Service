package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoutesExposesOnlyMetrics(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil)

	metrics := httptest.NewRecorder()
	mux.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", metrics.Code, http.StatusOK)
	}

	for _, path := range []string{"/v1/wallet", "/v1/wallet/abc", "/v1/swagger/"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}
}
