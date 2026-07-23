package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestGeneratedJWTUsesFixedClockAndEdDSA(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	credentials, err := NewGeneratedJWT("project-1", "credential-1", privateKey, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 23, 1, 2, 3, 0, time.UTC)
	header, err := credentials.Header(now)
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != "Authorization" || !strings.HasPrefix(header.Value, "Bearer ") {
		t.Fatalf("header = %#v", header)
	}
	parts := strings.Split(strings.TrimPrefix(header.Value, "Bearer "), ".")
	if len(parts) != 3 {
		t.Fatalf("JWT parts = %d", len(parts))
	}
	decode := func(part string, destination any) {
		t.Helper()
		contents, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(contents, destination); err != nil {
			t.Fatal(err)
		}
	}
	var gotHeader map[string]any
	var gotPayload map[string]any
	decode(parts[0], &gotHeader)
	decode(parts[1], &gotPayload)
	if gotHeader["alg"] != "EdDSA" || gotHeader["kid"] != "credential-1" {
		t.Fatalf("JWT header = %#v", gotHeader)
	}
	if gotPayload["sub"] != "project-1" || int64(gotPayload["iat"].(float64)) != now.Add(-30*time.Second).Unix() || int64(gotPayload["exp"].(float64)) != now.Add(-30*time.Second).Add(15*time.Minute).Unix() {
		t.Fatalf("JWT payload = %#v", gotPayload)
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(privateKey.Public().(ed25519.PublicKey), []byte(unsigned), signature) {
		t.Fatal("JWT signature is invalid")
	}

	second, err := credentials.Header(now)
	if err != nil {
		t.Fatal(err)
	}
	if second.Value != header.Value {
		t.Fatal("fixed clock produced a different JWT")
	}
}

func TestParsePrivateKeyPEM(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	contents := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	parsed, err := ParsePrivateKeyPEM(contents)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Equal(privateKey) {
		t.Fatal("parsed key differs")
	}
}

func TestCredentialsRejectInvalidInputs(t *testing.T) {
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	if _, err := NewGeneratedJWT("project", "credential", privateKey, MaxJWTTTL+time.Second); err == nil {
		t.Fatal("overlong JWT TTL was accepted")
	}
	if _, err := NewExternalJWT("has spaces"); err == nil {
		t.Fatal("invalid external JWT was accepted")
	}
	if _, err := NewAPIKey("\n"); err == nil {
		t.Fatal("invalid API key was accepted")
	}
}
