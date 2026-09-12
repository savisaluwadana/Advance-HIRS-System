package auth

import (
	"testing"
	"time"
)

func TestManagerIssueAndParse(t *testing.T) {
	manager, err := NewManager("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, expires, err := manager.Issue("user-1", "admin@example.com", "Admin", "org-1", "Northstar", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(expires) <= 0 {
		t.Fatal("expected token expiry in the future")
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.OrganizationID != "org-1" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewManager("too-short", time.Hour); err == nil {
		t.Fatal("expected short JWT secret to be rejected")
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("a-strong-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "a-strong-test-password") {
		t.Fatal("expected password to match hash")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to be rejected")
	}
}
