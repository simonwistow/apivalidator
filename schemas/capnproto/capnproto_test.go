package capnproto_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	capnp "capnproto.org/go/capnp/v3"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/capnproto"
)

const addressBookSchema = `
@0xb5e5d3aee89e1397;

struct Person {
  name @0 :Text;
  email @1 :Text;
}

struct AddressBook {
  people @0 :List(Person);
}
`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatCapnProto, []byte(addressBookSchema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

// buildCapnpMessage creates a minimal valid Cap'n Proto message using the
// capnp library's low-level API. This produces a real binary message that
// the validator can structurally decode.
func buildCapnpMessage(t *testing.T) []byte {
	t.Helper()
	arena := capnp.SingleSegment(nil)
	msg, seg, err := capnp.NewMessage(arena)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	// Allocate a simple struct (8 bytes data, 0 pointers — a generic placeholder).
	st, err := capnp.NewRootStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 0})
	if err != nil {
		t.Fatalf("NewRootStruct: %v", err)
	}
	_ = st
	b, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return b
}

func TestCapnProto_SchemaParse(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatCapnProto, addressBookSchema)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
}

func TestCapnProto_ValidBinaryMessage(t *testing.T) {
	v := newValidator(t)
	body := buildCapnpMessage(t)
	req := httptest.NewRequest(http.MethodPost, "/address", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid Cap'n Proto message, got: %s", res.Error())
	}
}

func TestCapnProto_TruncatedMessage(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/address",
		bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0xff, 0xff})) // invalid framing
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for truncated/malformed Cap'n Proto message")
	}
}

func TestCapnProto_EmptyBody(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/address", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for empty body, got: %s", res.Error())
	}
}
