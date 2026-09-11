package fieldcipher

import "testing"

func TestRoundTrip(t *testing.T) {
	Init("test-passphrase")
	t.Cleanup(func() { Init("") })

	enc, err := Encrypt("merchant-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "merchant-secret-key" {
		t.Fatal("expected encrypted value")
	}
	plain, err := Decrypt(enc)
	if err != nil || plain != "merchant-secret-key" {
		t.Fatalf("got %q err=%v", plain, err)
	}
}

func TestPlaintextPassthrough(t *testing.T) {
	Init("")
	plain, err := Decrypt("legacy-plain-key")
	if err != nil || plain != "legacy-plain-key" {
		t.Fatalf("legacy passthrough failed: %q err=%v", plain, err)
	}
}
