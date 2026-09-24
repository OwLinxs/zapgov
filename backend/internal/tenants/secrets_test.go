package tenants

import (
	"strings"
	"testing"
)

func TestCredentialEncryption(t *testing.T) {
	t.Setenv("META_CREDENTIALS_KEY", strings.Repeat("42", 32))
	original := "test-token-not-real"
	sealed, err := sealSecret(original, "tenant:2:token")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, original) {
		t.Fatal("plaintext leaked")
	}
	plain, err := openSecret(sealed, "tenant:2:token")
	if err != nil || plain != original {
		t.Fatalf("roundtrip: %v", err)
	}
	if _, err := openSecret(sealed, "tenant:3:token"); err == nil {
		t.Fatal("credential transferable across tenants")
	}
	t.Setenv("META_CREDENTIALS_KEY", strings.Repeat("43", 32))
	if _, err := openSecret(sealed, "tenant:2:token"); err == nil {
		t.Fatal("wrong key accepted")
	}
}
