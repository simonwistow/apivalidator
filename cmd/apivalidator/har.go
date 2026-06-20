package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// harLog is the top-level structure of an HTTP Archive (HAR) file.
type harLog struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	Request  harRequest  `json:"request"`
	Response harResponse `json:"response"`
}

type harRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []harHeader  `json:"headers"`
	QueryString []harNVP     `json:"queryString"`
	PostData    *harPostData `json:"postData"`
}

type harResponse struct {
	Status  int         `json:"status"`
	Headers []harHeader `json:"headers"`
	Content harContent  `json:"content"`
}

type harHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harNVP struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type harContent struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// loadHAR reads and parses a HAR file from disk.
func loadHAR(path string) (*harLog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HAR file: %w", err)
	}
	var log harLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("parse HAR JSON: %w", err)
	}
	return &log, nil
}

// toHTTPRequest converts a HAR request entry into an *http.Request.
func (hr *harRequest) toHTTPRequest() (*http.Request, error) {
	u, err := url.Parse(hr.URL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", hr.URL, err)
	}

	var body io.Reader
	if hr.PostData != nil && hr.PostData.Text != "" {
		body = strings.NewReader(hr.PostData.Text)
	}

	req, err := http.NewRequest(hr.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	for _, h := range hr.Headers {
		req.Header.Set(h.Name, h.Value)
	}

	// Ensure Host is set (required for routing).
	if req.Host == "" {
		req.Host = u.Host
	}

	return req, nil
}

// toHTTPResponse converts a HAR response entry into an *http.Response.
func (hr *harResponse) toHTTPResponse(req *http.Request) *http.Response {
	resp := &http.Response{
		StatusCode: hr.Status,
		Header:     make(http.Header),
		Request:    req,
	}
	for _, h := range hr.Headers {
		resp.Header.Set(h.Name, h.Value)
	}
	if hr.Content.Text != "" {
		resp.Body = io.NopCloser(bytes.NewReader([]byte(hr.Content.Text)))
	} else {
		resp.Body = http.NoBody
	}
	return resp
}
