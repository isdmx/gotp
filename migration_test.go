package main

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func appendBytesField(b []byte, num protowire.Number, v []byte) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

func appendVarintField(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// buildTestPayload constructs a valid MigrationPayload protobuf with a single
// OtpParameters entry, mirroring the wire format Google Authenticator exports.
func buildTestPayload() []byte {
	var op []byte
	op = appendBytesField(op, 1, []byte("Hello!"))
	op = appendBytesField(op, 2, []byte("user@example.com"))
	op = appendBytesField(op, 3, []byte("Google"))
	op = appendVarintField(op, 4, uint64(AlgoSHA1))
	op = appendVarintField(op, 5, 6)
	op = appendVarintField(op, 6, uint64(OtpTOTP))

	var payload []byte
	payload = appendBytesField(payload, 1, op)
	payload = appendVarintField(payload, 2, 1)
	payload = appendVarintField(payload, 3, 1)
	payload = appendVarintField(payload, 4, 0)
	payload = appendVarintField(payload, 5, 1)
	return payload
}

func TestParseMigrationPayload(t *testing.T) {
	payload, err := parseMigrationPayload(buildTestPayload())
	if err != nil {
		t.Fatalf("parseMigrationPayload returned error: %v", err)
	}

	if len(payload.OtpParameters) != 1 {
		t.Fatalf("got %d otp parameters, want 1", len(payload.OtpParameters))
	}
	op := payload.OtpParameters[0]
	if string(op.Secret) != "Hello!" {
		t.Errorf("secret = %q, want %q", op.Secret, "Hello!")
	}
	if op.Name != "user@example.com" {
		t.Errorf("name = %q, want %q", op.Name, "user@example.com")
	}
	if op.Issuer != "Google" {
		t.Errorf("issuer = %q, want %q", op.Issuer, "Google")
	}
	if op.Algorithm != AlgoSHA1 {
		t.Errorf("algorithm = %d, want %d", op.Algorithm, AlgoSHA1)
	}
	if op.Digits != 6 {
		t.Errorf("digits = %d, want 6", op.Digits)
	}
	if op.Type != OtpTOTP {
		t.Errorf("type = %d, want %d", op.Type, OtpTOTP)
	}
	if payload.Version != 1 {
		t.Errorf("version = %d, want 1", payload.Version)
	}
	if payload.BatchSize != 1 {
		t.Errorf("batch size = %d, want 1", payload.BatchSize)
	}
	if payload.BatchID != 1 {
		t.Errorf("batch id = %d, want 1", payload.BatchID)
	}
}

func TestParseMigrationPayloadEmpty(t *testing.T) {
	payload, err := parseMigrationPayload(nil)
	if err != nil {
		t.Fatalf("parseMigrationPayload(nil) returned error: %v", err)
	}
	if len(payload.OtpParameters) != 0 {
		t.Fatalf("got %d otp parameters, want 0", len(payload.OtpParameters))
	}
}

func TestEncodeSecret(t *testing.T) {
	got := encodeSecret([]byte("Hello!"))
	want := "JBSWY3DPEE"
	if got != want {
		t.Errorf("encodeSecret = %q, want %q", got, want)
	}
}

func TestFormatOTPCodes(t *testing.T) {
	p := MigrationPayload{
		OtpParameters: []OtpParameter{
			{Issuer: "Google", Name: "user@example.com", Secret: []byte("Hello!")},
		},
	}
	want := "Issuer: Google\n" +
		"Name: user@example.com\n" +
		"Secret: JBSWY3DPEE\n" +
		"-----------------------------------\n"
	if got := formatOTPCodes(p); got != want {
		t.Errorf("formatOTPCodes = %q, want %q", got, want)
	}
}
