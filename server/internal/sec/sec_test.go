package sec

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse", hash) {
		t.Fatal("expected match")
	}
	if VerifyPassword("wrong battery", hash) {
		t.Fatal("expected mismatch")
	}
	if VerifyPassword("correct horse", "not-argon") {
		t.Fatal("invalid encoding")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Foo@Bar.COM "); got != "foo@bar.com" {
		t.Fatalf("got %q", got)
	}
}

func TestHashToken(t *testing.T) {
	a := HashToken("abc")
	b := HashToken("abc")
	c := HashToken("abd")
	if a != b {
		t.Fatal("same token must hash equally")
	}
	if a == c {
		t.Fatal("different tokens must not collide")
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex length: %d", len(a))
	}
}
