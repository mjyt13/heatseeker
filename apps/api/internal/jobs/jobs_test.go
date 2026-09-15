package jobs

import "testing"

func TestParseQueues(t *testing.T) {
	got, err := ParseQueues("critical:6, default:3,low:1")
	if err != nil {
		t.Fatal(err)
	}
	if got["critical"] != 6 || got["default"] != 3 || got["low"] != 1 {
		t.Fatalf("got %v", got)
	}
	if _, err := ParseQueues("bad"); err == nil {
		t.Fatal("expected error for missing weight")
	}
	if _, err := ParseQueues("x:0"); err == nil {
		t.Fatal("expected error for zero weight")
	}
	def, err := ParseQueues("")
	if err != nil || def["default"] != 1 {
		t.Fatalf("empty spec should default, got %v %v", def, err)
	}
}
