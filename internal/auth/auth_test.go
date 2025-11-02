package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT_Success(t *testing.T) {
	id := uuid.New()
	secret := "test-secret"
	tok, err := MakeJWT(id, secret, time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT error: %v", err)
	}
	got, err := ValidateJWT(tok, secret)
	if err != nil {
		t.Fatalf("ValidateJWT error: %v", err)
	}
	if got != id {
		t.Fatalf("want %s, got %s", id, got)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	id := uuid.New()
	secret := "test-secret"
	tok, err := MakeJWT(id, secret, -time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT error: %v", err)
	}
	if _, err := ValidateJWT(tok, secret); err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	id := uuid.New()
	tok, err := MakeJWT(id, "right-secret", time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT error: %v", err)
	}
	if _, err := ValidateJWT(tok, "wrong-secret"); err == nil {
		t.Fatalf("expected error for wrong secret")
	}
}
