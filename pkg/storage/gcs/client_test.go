package gcs

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSignedURLSuccess(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	client := &Client{
		defaultBucket: "bucket",
		serviceAccount: &serviceAccountInfo{
			clientEmail: "signer@example.com",
			privateKey:  key,
		},
	}

	object := "media/product/file.png"
	contentType := "image/png"
	urlStr, err := client.SignedURL("bucket", object, contentType, 5*time.Minute)
	if err != nil {
		t.Fatalf("SignedURL returned error: %v", err)
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse signed url: %v", err)
	}

	if !strings.EqualFold(parsed.Host, "storage.googleapis.com") {
		t.Fatalf("unexpected host %s", parsed.Host)
	}

	values := parsed.Query()
	if got := values.Get("GoogleAccessId"); got != "signer@example.com" {
		t.Fatalf("unexpected GoogleAccessId %q", got)
	}

	expireParam := values.Get("Expires")
	if expireParam == "" {
		t.Fatalf("Expires missing")
	}
	expiration, err := strconv.ParseInt(expireParam, 10, 64)
	if err != nil {
		t.Fatalf("parse expires: %v", err)
	}
	expireParam = strconv.FormatInt(expiration, 10)

	signature := values.Get("Signature")
	if signature == "" {
		t.Fatalf("signature missing")
	}

	data := []byte("PUT\n\n" + contentType + "\n" + expireParam + "\n/" + "bucket" + "/" + object)
	hash := sha256.Sum256(data)

	rawSig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], rawSig); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
}

func TestSignedURLErrors(t *testing.T) {
	t.Parallel()

	client := &Client{
		serviceAccount: &serviceAccountInfo{
			clientEmail: "test@example.com",
			privateKey:  mustGenerateKey(t),
		},
		defaultBucket: "bucket",
	}

	cases := []struct {
		name              string
		bucket            string
		object            string
		contentType       string
		expires           time.Duration
		forceClearDefault bool
	}{
		{"missing bucket", "", "object", "image/png", time.Minute, true},
		{"missing object", "bucket", "", "image/png", time.Minute, false},
		{"missing contentType", "bucket", "object", "", time.Minute, false},
		{"negative ttl", "bucket", "object", "image/png", -time.Minute, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origBucket := client.defaultBucket
			if tc.forceClearDefault {
				client.defaultBucket = ""
			}
			defer func() {
				client.defaultBucket = origBucket
			}()
			if _, err := client.SignedURL(tc.bucket, tc.object, tc.contentType, tc.expires); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}

	emptyClient := &Client{}
	if _, err := emptyClient.SignedURL("", "object", "image/png", time.Minute); err == nil {
		t.Fatal("expected error without service account")
	}
}

func mustGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}
