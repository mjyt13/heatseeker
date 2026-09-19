package secretbox

import (
	"bytes"
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	box, err := New(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	owner := []byte("group-1")
	sealed, err := box.Seal([]byte("1//refresh-token"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("refresh")) {
		t.Fatal("plaintext leaked into the sealed value")
	}
	again, _ := box.Seal([]byte("1//refresh-token"), owner)
	if bytes.Equal(sealed, again) {
		t.Fatal("nonce is not random")
	}
	got, err := box.Open(sealed, owner)
	if err != nil || string(got) != "1//refresh-token" {
		t.Fatalf("open = %q, %v", got, err)
	}

	if _, err := box.Open(sealed, []byte("group-2")); err == nil {
		t.Error("opened with another owner")
	}
	other, _ := New(strings.Repeat("x", 32))
	if _, err := other.Open(sealed, owner); err == nil {
		t.Error("opened with another key")
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Open(tampered, owner); err == nil {
		t.Error("opened a tampered value")
	}
	if _, err := box.Open([]byte("short"), owner); err == nil {
		t.Error("opened a truncated value")
	}
	if _, err := New("short"); err == nil {
		t.Error("accepted a short key")
	}
}
