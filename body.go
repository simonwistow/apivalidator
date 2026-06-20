package apivalidator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// ReadRequestBody reads up to cfg.MaxBodyBytes from req.Body and replaces it
// with a re-readable buffer. Called by schema implementations that need to
// inspect the request body without consuming it.
func ReadRequestBody(req *http.Request, cfg *Config) ([]byte, error) {
	return readAndReplaceBody(req, cfg.MaxBodyBytes)
}

// ReadResponseBody reads up to cfg.MaxBodyBytes from resp.Body and replaces it
// with a re-readable buffer.
func ReadResponseBody(resp *http.Response, cfg *Config) ([]byte, error) {
	return readAndReplaceResponseBody(resp, cfg.MaxBodyBytes)
}

// readAndReplaceBody reads up to maxBytes from r, then replaces r with a
// re-readable buffer containing the same bytes. Returns the buffered bytes.
//
// This is required for binary-protocol validators (gRPC, CapnProto, Thrift)
// and JSON body validators that need to read the body without consuming it.
// The pattern is identical to what many HTTP middleware libraries use.
func readAndReplaceBody(r *http.Request, maxBytes int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	lr := io.LimitReader(r.Body, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrBodyTooLarge
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

// readAndReplaceResponseBody reads up to maxBytes from resp, then replaces
// resp.Body with a re-readable buffer.
func readAndReplaceResponseBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if resp.Body == nil {
		return nil, nil
	}
	lr := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrBodyTooLarge
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}
