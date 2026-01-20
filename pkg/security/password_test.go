package security_test

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"github.com/angelmondragon/packfinderz-backend/pkg/security"
)

func TestHashAndVerifyPassword(t *testing.T) {
	cfg := config.PasswordConfig{
		ArgonMemoryKB:    32768,
		ArgonTime:        1,
		ArgonParallelism: 1,
		ArgonSaltLen:     16,
		ArgonKeyLen:      32,
	}

	hash, err := security.HashPassword("very-secure-password", cfg)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}

	ok, err := security.VerifyPassword("very-secure-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error for valid hash: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword failed for the correct password")
	}

	ok, err = security.VerifyPassword("bogus-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error for invalid password: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword returned true for incorrect password")
	}
}

func TestVerifyPasswordBadHash(t *testing.T) {
	if _, err := security.VerifyPassword("irrelevant", "not-a-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}
