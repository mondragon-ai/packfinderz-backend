package gcs

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
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
	// if got := values.Get("GoogleAccessId"); got != "signer@example.com" {
	// 	t.Fatalf("unexpected GoogleAccessId %q", got)
	// }
	got := values.Get("GoogleAccessId")

	got, err = url.QueryUnescape(got)
	if err != nil {
		t.Fatalf("unescape GoogleAccessId: %v", err)
	}

	if got != "signer@example.com" {
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

	// rawSig, err := base64.RawURLEncoding.DecodeString(signature)
	// if err != nil {
	// 	t.Fatalf("decode signature: %v", err)
	// }
	signature, err = url.QueryUnescape(signature)
	if err != nil {
		t.Fatalf("unescape signature: %v", err)
	}

	rawSig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], rawSig); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
}

func TestSignedReadURLSuccess(t *testing.T) {
	t.Parallel()

	key := mustGenerateKey(t)
	client := &Client{
		defaultBucket: "bucket",
		serviceAccount: &serviceAccountInfo{
			clientEmail: "signer@example.com",
			privateKey:  key,
		},
	}

	object := "media/product/file.pdf"
	urlStr, err := client.SignedReadURL("bucket", object, 5*time.Minute)
	if err != nil {
		t.Fatalf("SignedReadURL returned error: %v", err)
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse signed read url: %v", err)
	}

	values := parsed.Query()
	expireParam := values.Get("Expires")
	if expireParam == "" {
		t.Fatal("Expires missing")
	}
	expiration, err := strconv.ParseInt(expireParam, 10, 64)
	if err != nil {
		t.Fatalf("parse expires: %v", err)
	}
	expireParam = strconv.FormatInt(expiration, 10)
	signature := values.Get("Signature")
	if signature == "" {
		t.Fatal("signature missing")
	}

	data := []byte("GET\n\n\n" + expireParam + "\n/" + "bucket" + "/" + object)
	hash := sha256.Sum256(data)
	// rawSig, err := base64.RawURLEncoding.DecodeString(signature)
	// if err != nil {
	// 	t.Fatalf("decode signature: %v", err)
	// }

	signature = strings.ReplaceAll(signature, " ", "+") // critical line

	rawSig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], rawSig); err != nil {
		t.Fatalf("verify read signature: %v", err)
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

type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestDeleteObjectSuccess(t *testing.T) {
	t.Parallel()

	key := mustGenerateKey(t)
	client := &Client{
		defaultBucket: "bucket",
		serviceAccount: &serviceAccountInfo{
			clientEmail: "signer@example.com",
			privateKey:  key,
		},
		tokenSource: &tokenSource{fetch: func(context.Context) (string, time.Time, error) {
			return "token", time.Now().Add(time.Hour), nil
		}},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) *http.Response {
			if req.Method != http.MethodDelete {
				t.Fatalf("expected DELETE, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("unexpected auth %s", req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})},
	}

	if err := client.DeleteObject(context.Background(), "bucket", "media/file.png"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
}

func TestDeleteObjectNotFound(t *testing.T) {
	t.Parallel()

	key := mustGenerateKey(t)
	client := &Client{
		defaultBucket: "bucket",
		serviceAccount: &serviceAccountInfo{
			clientEmail: "signer@example.com",
			privateKey:  key,
		},
		tokenSource: &tokenSource{fetch: func(context.Context) (string, time.Time, error) {
			return "token", time.Now().Add(time.Hour), nil
		}},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})},
	}

	if err := client.DeleteObject(context.Background(), "bucket", "media/file.png"); err != nil {
		t.Fatalf("DeleteObject not found should succeed: %v", err)
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
