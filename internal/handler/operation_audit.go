package handler

import (
	"context"
	"net/http"
	"strings"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/storage"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	// Informational responses do not commit the final status (101 does).
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// Advertise Flusher only when the wrapped writer really supports streaming.
type auditFlushResponseWriter struct {
	*auditResponseWriter
	flusher http.Flusher
}

func (w *auditFlushResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	w.flusher.Flush()
}

func OperationAuditMiddleware(next http.Handler, repo *storage.TrafficRepository, tokens *auth.TokenStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutating := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete
		if !mutating || !strings.HasPrefix(r.URL.Path, "/api/admin/") {
			next.ServeHTTP(w, r)
			return
		}
		recorder := &auditResponseWriter{ResponseWriter: w}
		var writer http.ResponseWriter = recorder
		if flusher, ok := w.(http.Flusher); ok {
			writer = &auditFlushResponseWriter{auditResponseWriter: recorder, flusher: flusher}
		}
		next.ServeHTTP(writer, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		actor, _ := tokens.Lookup(strings.TrimSpace(r.Header.Get(auth.AuthHeader)))
		_ = repo.InsertOperationLog(context.Background(), storage.OperationLog{Actor: actor, Method: r.Method, Path: r.URL.Path, Status: recorder.status, IP: GetClientIP(r)})
	})
}
