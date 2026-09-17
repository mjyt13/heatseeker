package media

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"heatseeker/api/internal/domain"
)

func TestLocalStore(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "tmp/uploads/1/a.txt", strings.NewReader("hello world"), 11, "text/plain"); err != nil {
		t.Fatal(err)
	}
	info, err := store.Stat(ctx, "tmp/uploads/1/a.txt")
	if err != nil || info.Size != 11 {
		t.Fatalf("stat: %v %+v", err, info)
	}
	r, _, err := store.Open(ctx, "tmp/uploads/1/a.txt", 6, 3)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(r)
	_ = r.Close()
	if string(part) != "wor" {
		t.Fatalf("range read = %q", part)
	}
	if err := store.Move(ctx, "tmp/uploads/1/a.txt", "groups/g/materials/m/v1/a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, "tmp/uploads/1/a.txt"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("source still present: %v", err)
	}
	r, _, err = store.Open(ctx, "groups/g/materials/m/v1/a.txt", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := io.ReadAll(r)
	_ = r.Close()
	if string(all) != "hello world" {
		t.Fatalf("read = %q", all)
	}
	// keys cannot escape the root
	if err := store.Put(ctx, "../../etc/x", strings.NewReader("x"), 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, "etc/x"); err != nil {
		t.Fatalf("traversal key was not confined to the root: %v", err)
	}
	if err := store.Delete(ctx, "groups/g/materials/m/v1/a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "missing"); err != nil {
		t.Fatalf("deleting a missing object: %v", err)
	}
	if p, err := store.PresignPut(ctx, "k", "", 0); p != nil || err != nil {
		t.Fatalf("local store must not presign: %v %v", p, err)
	}
}
