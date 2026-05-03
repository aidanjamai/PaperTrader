package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches panics from downstream handlers, logs the stack trace at
// slog.Error with the request_id (if RequestLogger ran first), and returns a
// generic 500 response. Without this, a nil-pointer deref or out-of-bounds in
// any handler crashes the whole server process.
//
// Mount order in main.go is: RequestLogger → Recover → … → handlers. Mux's
// `Use` is FIFO, so the first registered middleware is the *outermost*
// wrapper. RequestLogger runs first so it can set the request_id; Recover
// runs immediately inside it so the recovery log line can read that ID and
// any panic in subsequent middleware (CORS, size-limit, timeout) is still
// caught. Do not register Recover before RequestLogger, or the recovery log
// will have no request_id.
func Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// http.ErrAbortHandler is the conventional way to abort a
				// handler without a stack trace (e.g. http.TimeoutHandler);
				// re-panic so the standard library's own recovery sees it.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				slog.Error("panic recovered",
					"request_id", RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"panic", rec,
					"stack", string(debug.Stack()),
				)

				// Best-effort response. If the handler already started writing
				// the body we can't change the status code, but we can still
				// stop the panic from killing the process.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success":    false,
					"message":    "internal server error",
					"error_code": "INTERNAL_ERROR",
				})
			}()
			next.ServeHTTP(w, r)
		})
	}
}
