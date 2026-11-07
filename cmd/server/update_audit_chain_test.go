package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/handler"
	"miaomiaowu/internal/storage"
)

func TestUpdateRejectionThroughRealMiddlewareChain(t *testing.T) {
	repo, err := storage.NewTrafficRepository(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenStore(time.Hour)
	credentials := map[string]string{}
	for _, role := range []string{"admin", "user"} {
		if err := repo.CreateUser(context.Background(), role, "", "", "unused", role, ""); err != nil {
			t.Fatal(err)
		}
		credentials[role], _, err = tokens.Issue(role)
		if err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	users := auth.NewRepositoryAdapter(repo)
	mux.Handle("/api/admin/update/apply-sse", auth.RequireAdmin(tokens, users, handler.NewUpdateApplySSEHandler()))
	mux.Handle("/api/admin/update/apply", auth.RequireAdmin(tokens, users, handler.NewUpdateApplyHandler()))
	// Same outer ordering and concrete middleware as main(): CORS -> silent ->
	// operation audit -> mux -> RequireAdmin -> update handler.
	chain := handler.OperationAuditMiddleware(mux, repo, tokens)
	chain = handler.NewSilentModeManager(repo, tokens).Middleware(chain)
	server := httptest.NewServer(withCORS(chain, []string{"http://test.invalid"}))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	for _, test := range []struct {
		name, path, role string
		status           int
	}{
		{"admin-sse", "/api/admin/update/apply-sse", "admin", 200},
		{"admin-apply", "/api/admin/update/apply", "admin", 503},
		{"user-forbidden", "/api/admin/update/apply-sse", "user", 403},
		{"missing-token", "/api/admin/update/apply-sse", "", 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+test.path, nil)
			req.Header.Set(auth.AuthHeader, credentials[test.role])
			req.Header.Set("Origin", "http://test.invalid")
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != test.status {
				t.Fatalf("status=%d body=%s err=%v", response.StatusCode, body, err)
			}
			if test.name == "admin-sse" && (!strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") || !strings.Contains(string(body), `"step":"checking"`) || !strings.Contains(string(body), `"step":"error"`) || !strings.Contains(string(body), "mmwx 暂未提供自有发行物") || strings.Contains(string(body), `"step":"downloading"`)) {
				t.Fatalf("invalid SSE rejection: %s", body)
			}
			logs, err := repo.ListOperationLogs(context.Background(), 1, 0)
			if err != nil || len(logs) != 1 || logs[0].Status != test.status || logs[0].Actor != test.role || logs[0].Path != test.path {
				t.Fatalf("audit=%+v err=%v", logs, err)
			}
		})
	}
}
