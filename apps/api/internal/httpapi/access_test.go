package httpapi

import "testing"

func TestInvitationTokensAreRandomAndStoredAsHashes(t *testing.T) {
	rawA, hashA, err := newInvitationToken()
	if err != nil {
		t.Fatal(err)
	}
	rawB, hashB, err := newInvitationToken()
	if err != nil {
		t.Fatal(err)
	}
	if rawA == rawB || hashA == hashB {
		t.Fatal("expected independently generated invitation tokens")
	}
	if rawA == hashA {
		t.Fatal("raw invitation token must not equal its persisted hash")
	}
	if len(hashA) != 64 {
		t.Fatalf("expected SHA-256 hex hash length 64, got %d", len(hashA))
	}
	if hashInvitationToken(rawA) != hashA {
		t.Fatal("expected invitation token hashing to be deterministic")
	}
}
