package apivalidator

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// MiddlewareOption configures the behaviour of Middleware.
type MiddlewareOption func(*middlewareCfg)

type middlewareCfg struct {
	onViolation      func(w http.ResponseWriter, r *http.Request, result *ValidationResult)
	passthroughCodes []int // HTTP status codes written by onViolation — unused; kept for future extension
	logViolations    bool
}

func defaultMiddlewareCfg() *middlewareCfg {
	return &middlewareCfg{
		onViolation:   defaultOnViolation,
		logViolations: true,
	}
}

// WithOnViolation sets a custom handler called when request validation fails.
// It replaces the default behaviour of writing a 400 JSON response.
// The handler is responsible for writing the HTTP response; the next handler
// in the chain will NOT be called.
func WithOnViolation(fn func(w http.ResponseWriter, r *http.Request, result *ValidationResult)) MiddlewareOption {
	return func(c *middlewareCfg) { c.onViolation = fn }
}

// WithLogViolations controls whether the middleware logs validation failures
// via the Validator's configured logger. Defaults to true.
func WithLogViolations(b bool) MiddlewareOption {
	return func(c *middlewareCfg) { c.logViolations = b }
}

// Middleware returns an HTTP middleware that validates incoming requests against
// the provided Validator. When validation fails, the configured onViolation
// handler is called (default: 400 JSON response) and the next handler is skipped.
// When ErrNoMatchingSchema is returned, the request passes through unchanged.
//
// Response validation is NOT performed by Middleware because it requires
// buffering the response body; use ValidateResponse explicitly when needed.
func Middleware(v *Validator, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := defaultMiddlewareCfg()
	for _, o := range opts {
		o(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := v.ValidateRequest(r)
			if err != nil {
				if errors.Is(err, ErrNoMatchingSchema) {
					// No schema covers this endpoint — pass through.
					next.ServeHTTP(w, r)
					return
				}
				// Unexpected error (e.g. ErrBodyTooLarge).
				if cfg.logViolations && v.cfg.Logger != nil {
					v.cfg.Logger.Error("apivalidator middleware error", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
				}
				http.Error(w, "request validation error: "+err.Error(), http.StatusBadRequest)
				return
			}
			if !result.Valid {
				if cfg.logViolations && v.cfg.Logger != nil {
					v.cfg.Logger.Warn("apivalidator request invalid",
						slog.String("path", r.URL.Path),
						slog.String("violations", result.Error()),
					)
				}
				cfg.onViolation(w, r, result)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// defaultOnViolation writes a 400 JSON response with the violation details.
func defaultOnViolation(w http.ResponseWriter, _ *http.Request, result *ValidationResult) {
	type violationJSON struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
		Path     string `json:"path,omitempty"`
		Message  string `json:"message"`
	}
	type responseJSON struct {
		Error      string          `json:"error"`
		Violations []violationJSON `json:"violations,omitempty"`
	}

	vs := make([]violationJSON, 0, len(result.Violations))
	for _, v := range result.Violations {
		vs = append(vs, violationJSON{
			Code:     string(v.Code),
			Severity: v.Severity.String(),
			Path:     v.Path,
			Message:  v.Message,
		})
	}
	resp := responseJSON{
		Error:      "request validation failed",
		Violations: vs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(resp)
}
