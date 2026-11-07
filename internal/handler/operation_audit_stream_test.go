package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"miaomiaowu/internal/auth"
)

// Hide Recorder's Flusher to check that the audit wrapper does not invent it.
type auditPlainWriter struct{ http.ResponseWriter }

func TestAuditWriterStreamingAndCommittedStatus(t *testing.T) {
	for _, name := range []string{"flush-first", "write-first", "explicit-status", "no-write", "no-flusher", "informational"} {
		t.Run(name, func(t *testing.T) {
			repo := relayTestRepo(t)
			tokens := auth.NewTokenStore(time.Hour)
			rec := httptest.NewRecorder()
			var underlying http.ResponseWriter = rec
			if name == "no-flusher" {
				underlying = auditPlainWriter{rec}
			}
			expected := 200
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				flusher, streaming := w.(http.Flusher)
				if streaming != (name != "no-flusher") {
					t.Fatal("wrapper changed streaming capability")
				}
				switch name {
				case "flush-first":
					flusher.Flush()
					w.WriteHeader(503) // Must not change the committed 200.
				case "write-first":
					_, _ = w.Write([]byte("event"))
					w.WriteHeader(503)
				case "explicit-status":
					expected = 503
					w.WriteHeader(expected)
					flusher.Flush()
					w.WriteHeader(200)
				case "informational":
					w.WriteHeader(103)
					expected = 201
					w.WriteHeader(expected)
				}
			})
			h := OperationAuditMiddleware(next, repo, tokens)
			h.ServeHTTP(underlying, httptest.NewRequest("POST", "/api/admin/example", nil))
			logs, err := repo.ListOperationLogs(context.Background(), 10, 0)
			if err != nil || len(logs) != 1 || logs[0].Status != expected {
				t.Fatalf("audit=%+v error=%v, expected %d", logs, err, expected)
			}
			// Recorder commits 103 as its only status, unlike the real server.
			if name != "informational" && rec.Code != expected {
				t.Fatalf("wire status=%d expected=%d", rec.Code, expected)
			}
			if name == "flush-first" && !rec.Flushed {
				t.Fatal("underlying writer was not flushed")
			}
		})
	}
}
