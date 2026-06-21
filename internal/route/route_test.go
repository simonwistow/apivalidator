package route_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/simonwistow/apivalidator/internal/route"
)

func makeReq(method, path, contentType string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func TestDetectTransport_GRPC(t *testing.T) {
	req := makeReq("POST", "/foo.Bar/Baz", "application/grpc")
	if got := route.DetectTransport(req); got != route.TransportGRPC {
		t.Errorf("expected TransportGRPC, got %v", got)
	}
}

func TestDetectTransport_GRPCWeb(t *testing.T) {
	for _, ct := range []string{"application/grpc-web", "application/grpc-web+proto"} {
		req := makeReq("POST", "/foo.Bar/Baz", ct)
		if got := route.DetectTransport(req); got != route.TransportGRPCWeb {
			t.Errorf("ct=%q: expected TransportGRPCWeb, got %v", ct, got)
		}
	}
}

func TestDetectTransport_GRPCWithParams(t *testing.T) {
	req := makeReq("POST", "/foo.Bar/Baz", "application/grpc; encoding=proto")
	if got := route.DetectTransport(req); got != route.TransportGRPC {
		t.Errorf("expected TransportGRPC with params stripped, got %v", got)
	}
}

func TestDetectTransport_Thrift(t *testing.T) {
	req := makeReq("POST", "/Service/method", "application/x-thrift")
	if got := route.DetectTransport(req); got != route.TransportThrift {
		t.Errorf("expected TransportThrift, got %v", got)
	}
}

func TestDetectTransport_GraphQLByCT(t *testing.T) {
	req := makeReq("POST", "/api", "application/graphql")
	if got := route.DetectTransport(req); got != route.TransportGraphQL {
		t.Errorf("expected TransportGraphQL, got %v", got)
	}
}

func TestDetectTransport_GraphQLByPath(t *testing.T) {
	for _, path := range []string{"/graphql", "/api/graphql"} {
		req := makeReq("POST", path, "application/json")
		if got := route.DetectTransport(req); got != route.TransportGraphQL {
			t.Errorf("path=%q: expected TransportGraphQL, got %v", path, got)
		}
	}
}

func TestDetectTransport_REST(t *testing.T) {
	req := makeReq("GET", "/users/1", "application/json")
	if got := route.DetectTransport(req); got != route.TransportREST {
		t.Errorf("expected TransportREST, got %v", got)
	}
}

func TestDetectTransport_NoCT(t *testing.T) {
	req := makeReq("GET", "/users/1", "")
	if got := route.DetectTransport(req); got != route.TransportREST {
		t.Errorf("expected TransportREST for no content-type, got %v", got)
	}
}

func TestDetectTransport_CaseInsensitive(t *testing.T) {
	req := makeReq("POST", "/foo.Bar/Baz", "Application/GRPC")
	if got := route.DetectTransport(req); got != route.TransportGRPC {
		t.Errorf("expected TransportGRPC (case-insensitive), got %v", got)
	}
}

func TestIsGRPCPath_Valid(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/foo.Bar/Baz", true},
		{"/package.sub.Service/Method", true}, // multi-dot service name, still 2 parts
		{"/Bar/Baz", false},                   // no dot in service
		{"/foo.Bar/", true},                   // empty method segment but still 2 parts + dot (implementation returns true)
		{"/foo.Bar", false},                   // only one part
		{"", false},
		{"/", false},
	}
	for _, tc := range cases {
		if got := route.IsGRPCPath(tc.path); got != tc.want {
			t.Errorf("IsGRPCPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGraphQL_PathExact(t *testing.T) {
	req := makeReq("POST", "/graphql", "application/json")
	if got := route.DetectTransport(req); got != route.TransportGraphQL {
		t.Errorf("expected TransportGraphQL for exact /graphql path, got %v", got)
	}
}

func TestRESTTransport_String(t *testing.T) {
	req := makeReq("DELETE", "/items/5", "")
	tr := route.DetectTransport(req)
	if tr == route.TransportGRPC || tr == route.TransportThrift || tr == route.TransportGraphQL {
		t.Errorf("DELETE /items/5 should not be gRPC/Thrift/GraphQL, got %v", tr)
	}
}
