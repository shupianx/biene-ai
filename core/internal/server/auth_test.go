package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareAllowsMatchingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(authHeaderName, "secret-token")

	called := false
	handler := authMiddleware("secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestAuthMiddlewareAllowsMatchingQueryToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/ws?token=query-secret", nil)

	called := false
	handler := authMiddleware("query-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)

	handler := authMiddleware("secret-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewarePassesThroughWhenDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)

	called := false
	handler := authMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
