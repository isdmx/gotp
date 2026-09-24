package main

import (
	"encoding/base32"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// base32NoPad is the RFC 4648 alphabet without '=' padding, matching the
// Base32 form that authenticator apps expect for shared secrets.
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// Algorithm mirrors MigrationPayload.OtpParameters.Algorithm.
type Algorithm int32

const (
	AlgoInvalid Algorithm = iota
	AlgoSHA1
)

// OtpType mirrors MigrationPayload.OtpParameters.OtpType.
type OtpType int32

const (
	OtpInvalid OtpType = iota
	OtpHOTP
	OtpTOTP
)

// OtpParameter holds one account's details from the migration payload.
type OtpParameter struct {
	Secret    []byte
	Name      string
	Issuer    string
	Algorithm Algorithm
	Digits    int32
	Type      OtpType
	Counter   int64
}

// MigrationPayload is the decoded Google Authenticator migration message.
type MigrationPayload struct {
	OtpParameters []OtpParameter
	Version       int32
	BatchSize     int32
	BatchIndex    int32
	BatchID       int32
}

func parseMigrationPayload(b []byte) (MigrationPayload, error) {
	var p MigrationPayload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return p, protowire.ParseError(n)
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			v, rest, err := consumeBytes(b)
			if err != nil {
				return p, err
			}
			b = rest
			op, err := parseOtpParameter(v)
			if err != nil {
				return p, err
			}
			p.OtpParameters = append(p.OtpParameters, op)
		case num == 2 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return p, err
			}
			b = rest
			p.Version = int32(v)
		case num == 3 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return p, err
			}
			b = rest
			p.BatchSize = int32(v)
		case num == 4 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return p, err
			}
			b = rest
			p.BatchIndex = int32(v)
		case num == 5 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return p, err
			}
			b = rest
			p.BatchID = int32(v)
		default:
			rest := protowire.ConsumeFieldValue(num, typ, b)
			if rest < 0 {
				return p, protowire.ParseError(rest)
			}
			b = b[rest:]
		}
	}
	return p, nil
}

func parseOtpParameter(b []byte) (OtpParameter, error) {
	var op OtpParameter
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return op, protowire.ParseError(n)
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			v, rest, err := consumeBytes(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Secret = v
		case num == 2 && typ == protowire.BytesType:
			v, rest, err := consumeBytes(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Name = string(v)
		case num == 3 && typ == protowire.BytesType:
			v, rest, err := consumeBytes(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Issuer = string(v)
		case num == 4 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Algorithm = Algorithm(v)
		case num == 5 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Digits = int32(v)
		case num == 6 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Type = OtpType(v)
		case num == 7 && typ == protowire.VarintType:
			v, rest, err := consumeVarint(b)
			if err != nil {
				return op, err
			}
			b = rest
			op.Counter = int64(v)
		default:
			rest := protowire.ConsumeFieldValue(num, typ, b)
			if rest < 0 {
				return op, protowire.ParseError(rest)
			}
			b = b[rest:]
		}
	}
	return op, nil
}

func consumeVarint(b []byte) (uint64, []byte, error) {
	v, n := protowire.ConsumeVarint(b)
	if n < 0 {
		return 0, b, protowire.ParseError(n)
	}
	return v, b[n:], nil
}

func consumeBytes(b []byte) ([]byte, []byte, error) {
	v, n := protowire.ConsumeBytes(b)
	if n < 0 {
		return nil, b, protowire.ParseError(n)
	}
	return v, b[n:], nil
}

// encodeSecret returns the RFC 4648 Base32 encoding of a raw OTP secret,
// unpadded, matching the representation other authenticators expect.
func encodeSecret(secret []byte) string {
	return base32NoPad.EncodeToString(secret)
}

// formatOTPCodes renders the decoded accounts in the original tool's text form.
func formatOTPCodes(p MigrationPayload) string {
	var sb strings.Builder
	for _, op := range p.OtpParameters {
		fmt.Fprintf(&sb, "Issuer: %s\n", op.Issuer)
		fmt.Fprintf(&sb, "Name: %s\n", op.Name)
		fmt.Fprintf(&sb, "Secret: %s\n", encodeSecret(op.Secret))
		sb.WriteString("-----------------------------------\n")
	}
	return sb.String()
}
