package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

const MaxJWTTTL = 24 * time.Hour

type Method string

const (
	MethodGeneratedJWT Method = "generated-jwt"
	MethodExternalJWT  Method = "external-jwt"
	MethodAPIKey       Method = "api-key"
)

type Credentials struct {
	method       Method
	projectID    string
	credentialID string
	privateKey   ed25519.PrivateKey
	ttl          time.Duration
	token        string
	apiKey       string
}

type Header struct {
	Name  string
	Value string
}

func NewGeneratedJWT(projectID, credentialID string, privateKey ed25519.PrivateKey, ttl time.Duration) (Credentials, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(credentialID) == "" {
		return Credentials{}, errors.New("project ID and credential ID are required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Credentials{}, errors.New("Ed25519 private key has an invalid size")
	}
	if ttl <= 0 || ttl > MaxJWTTTL {
		return Credentials{}, fmt.Errorf("JWT TTL must be positive and no greater than %s", MaxJWTTTL)
	}
	return Credentials{
		method:       MethodGeneratedJWT,
		projectID:    projectID,
		credentialID: credentialID,
		privateKey:   append(ed25519.PrivateKey(nil), privateKey...),
		ttl:          ttl,
	}, nil
}

func NewExternalJWT(token string) (Credentials, error) {
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return Credentials{}, errors.New("external JWT must be a non-empty compact token")
	}
	return Credentials{method: MethodExternalJWT, token: token}, nil
}

func NewAPIKey(value string) (Credentials, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return Credentials{}, errors.New("API key must be non-empty and single-line")
	}
	return Credentials{method: MethodAPIKey, apiKey: value}, nil
}

func (c Credentials) Method() Method {
	return c.method
}

func (c Credentials) Header(now time.Time) (Header, error) {
	switch c.method {
	case MethodGeneratedJWT:
		token, err := c.sign(now)
		if err != nil {
			return Header{}, err
		}
		return Header{Name: "Authorization", Value: "Bearer " + token}, nil
	case MethodExternalJWT:
		return Header{Name: "Authorization", Value: "Bearer " + c.token}, nil
	case MethodAPIKey:
		return Header{Name: "X-QW-Api-Key", Value: c.apiKey}, nil
	default:
		return Header{}, errors.New("authentication method is not configured")
	}
}

func (c Credentials) sign(now time.Time) (string, error) {
	if len(c.privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("Ed25519 private key is unavailable")
	}
	headerJSON, err := json.Marshal(struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}{Algorithm: "EdDSA", KeyID: c.credentialID})
	if err != nil {
		return "", fmt.Errorf("encode JWT header: %w", err)
	}
	issuedAt := now.Add(-30 * time.Second)
	payloadJSON, err := json.Marshal(struct {
		Subject   string `json:"sub"`
		IssuedAt  int64  `json:"iat"`
		ExpiresAt int64  `json:"exp"`
	}{Subject: c.projectID, IssuedAt: issuedAt.Unix(), ExpiresAt: issuedAt.Add(c.ttl).Unix()})
	if err != nil {
		return "", fmt.Errorf("encode JWT payload: %w", err)
	}
	encoding := base64.RawURLEncoding
	unsigned := encoding.EncodeToString(headerJSON) + "." + encoding.EncodeToString(payloadJSON)
	signature := ed25519.Sign(c.privateKey, []byte(unsigned))
	return unsigned + "." + encoding.EncodeToString(signature), nil
}

func ParsePrivateKeyPEM(contents []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("private key is not PEM encoded")
	}
	if len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("private key file contains trailing data")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return append(ed25519.PrivateKey(nil), key...), nil
}
