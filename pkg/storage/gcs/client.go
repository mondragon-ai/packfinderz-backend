package gcs

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

const (
	tokenEndpoint = "https://oauth2.googleapis.com/token"
	scope         = "https://www.googleapis.com/auth/devstorage.read_write"
	pingTimeout   = 5 * time.Second
	metadataToken = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
)

type Client struct {
	httpClient    *http.Client
	defaultBucket string
	tokenSource   *tokenSource
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func closeBody(ctx context.Context, logg *logger.Logger, body io.Closer, msg string) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil && logg != nil {
		logg.Warn(ctx, msg)
	}
}

func NewClient(ctx context.Context, cfg config.GCSConfig, gcp config.GCPConfig, logg *logger.Logger) (*Client, error) {
	if cfg.BucketName == "" {
		return nil, errors.New("gcs bucket name is required")
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	var ts *tokenSource
	var err error
	switch {
	case gcp.CredentialsJSON != "":
		ts, err = newServiceAccountTokenSource(httpClient, gcp.CredentialsJSON)
	case gcp.ApplicationCredentials != "":
		bytes, readErr := os.ReadFile(gcp.ApplicationCredentials)
		if readErr != nil {
			return nil, fmt.Errorf("reading credentials file: %w", readErr)
		}
		ts, err = newServiceAccountTokenSource(httpClient, string(bytes))
	default:
		ts = newMetadataTokenSource(httpClient)
	}
	if err != nil {
		return nil, err
	}

	client := &Client{
		httpClient:    httpClient,
		defaultBucket: cfg.BucketName,
		tokenSource:   ts,
	}

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("gcs health check failed: %w", err)
	}

	if logg != nil {
		logg.Info(ctx, "gcs client initialized")
	}

	return client, nil
}

func (c *Client) BucketHandle(name string) *Bucket {
	if c == nil {
		return nil
	}
	if name == "" {
		name = c.defaultBucket
	}
	return &Bucket{name: name, client: c}
}

func (c *Client) DefaultBucket() string {
	if c == nil {
		return ""
	}
	return c.defaultBucket
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.tokenSource == nil {
		return errors.New("gcs client not initialized")
	}
	if c.defaultBucket == "" {
		return errors.New("gcs bucket not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return err
	}

	// Prefer object-level check (requires storage.objects.list)
	u := fmt.Sprintf(
		"https://storage.googleapis.com/storage/v1/b/%s/o?maxResults=1",
		url.PathEscape(c.defaultBucket),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// read a tiny bit of body for better debugging (optional)
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(b) > 0 {
			return fmt.Errorf("gcs object check failed: %s: %s", resp.Status, strings.TrimSpace(string(b)))
		}
		return fmt.Errorf("gcs object check failed: %s", resp.Status)
	}

	return nil
}

type Bucket struct {
	name   string
	client *Client
}

func (b *Bucket) Name() string {
	return b.name
}

type tokenSource struct {
	mu     sync.Mutex
	token  string
	expiry time.Time
	fetch  func(context.Context) (string, time.Time, error)
}

func (t *tokenSource) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token != "" && time.Until(t.expiry) > time.Minute {
		return t.token, nil
	}

	token, expiry, err := t.fetch(ctx)
	if err != nil {
		return "", err
	}
	t.token = token
	t.expiry = expiry
	return token, nil
}

func newServiceAccountTokenSource(client *http.Client, jsonCreds string) (*tokenSource, error) {
	var creds struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(jsonCreds), &creds); err != nil {
		return nil, fmt.Errorf("parsing service account credentials: %w", err)
	}
	if creds.ClientEmail == "" || creds.PrivateKey == "" {
		return nil, errors.New("invalid service account credentials")
	}
	tokenURI := creds.TokenURI
	if tokenURI == "" {
		tokenURI = tokenEndpoint
	}
	priv, err := parsePrivateKey(creds.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &tokenSource{
		fetch: func(ctx context.Context) (string, time.Time, error) {
			return fetchServiceAccountToken(ctx, client, creds.ClientEmail, priv, tokenURI)
		},
	}, nil
}

func newMetadataTokenSource(client *http.Client) *tokenSource {
	return &tokenSource{
		fetch: func(ctx context.Context) (string, time.Time, error) {
			return fetchMetadataToken(ctx, client)
		},
	}
}

func fetchServiceAccountToken(ctx context.Context, client *http.Client, email string, key *rsa.PrivateKey, tokenURI string) (string, time.Time, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	now := time.Now()
	claims := map[string]any{
		"iss":   email,
		"scope": scope,
		"aud":   tokenURI,
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := strings.Join([]string{header, payload}, ".")
	signature, err := signJWT(unsigned, key)
	if err != nil {
		return "", time.Time{}, err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", unsigned+"."+signature)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer func() { closeBody(ctx, nil, resp.Body, "gcs: closing response body failed") }()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token endpoint returned %s", resp.Status)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}

	return tokenResp.AccessToken, time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

func fetchMetadataToken(ctx context.Context, client *http.Client) (string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataToken, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}

	defer func() { closeBody(ctx, nil, resp.Body, "gcs: closing response body failed") }()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("metadata token request returned %s", resp.Status)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}

	return tokenResp.AccessToken, time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("invalid private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if priv, ok := key.(*rsa.PrivateKey); ok {
			return priv, nil
		}
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("unsupported private key format")
	}
	return priv, nil
}

func signJWT(unsigned string, key *rsa.PrivateKey) (string, error) {
	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(signature), nil
}
