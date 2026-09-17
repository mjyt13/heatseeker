package materials

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		header          string
		size            int64
		offset, length  int64
		partial, failed bool
	}{
		{"", 100, 0, 100, false, false},
		{"bytes=0-9", 100, 0, 10, true, false},
		{"bytes=90-", 100, 90, 10, true, false},
		{"bytes=90-500", 100, 90, 10, true, false},
		{"bytes=-5", 100, 95, 5, true, false},
		{"bytes=-500", 100, 0, 100, true, false},
		{"bytes=100-", 100, 0, 0, false, true},
		{"bytes=5-1", 100, 0, 0, false, true},
		{"bytes=-0", 100, 0, 0, false, true},
		{"bytes=abc-", 100, 0, 0, false, true},
		{"bytes=0-1,5-6", 100, 0, 100, false, false}, // multipart ranges: whole file
		{"items=0-1", 100, 0, 100, false, false},
	}
	for _, tt := range tests {
		offset, length, partial, err := parseRange(tt.header, tt.size)
		if tt.failed {
			if !IsRangeError(err) {
				t.Errorf("%q: expected range error, got %v", tt.header, err)
			}
			continue
		}
		if err != nil || offset != tt.offset || length != tt.length || partial != tt.partial {
			t.Errorf("%q: got (%d, %d, %v, %v), want (%d, %d, %v)", tt.header, offset, length, partial, err, tt.offset, tt.length, tt.partial)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	c := domain.MaterialCursor{SortAt: time.Date(2026, 9, 16, 10, 11, 12, 123456000, time.UTC), ID: uuid.New()}
	got, err := decodeCursor(encodeCursor(c))
	if err != nil || !got.SortAt.Equal(c.SortAt) || got.ID != c.ID {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	for _, bad := range []string{"", "!!", "AAAA"} {
		if _, err := decodeCursor(bad); err == nil {
			t.Errorf("decodeCursor(%q) accepted", bad)
		}
	}
}

func TestMarkManual(t *testing.T) {
	low := domain.ReviewLowConfidence
	subject := uuid.New()
	p := domain.UpdateMaterialParams{SubjectID: &subject, Kind: domain.KindLecture, NeedsReview: true, ReviewReason: &low,
		Classification: domain.Classification{Method: domain.ClassifyAuto, Confidence: 0.3}}
	markManual(&p)
	if p.NeedsReview || p.ReviewReason != nil || p.Classification.Method != domain.ClassifyManual || *p.Classification.SubjectID != subject {
		t.Fatalf("low-confidence review must be resolved: %+v", p)
	}
	removed := domain.ReviewRemovedFromDrive
	p = domain.UpdateMaterialParams{NeedsReview: true, ReviewReason: &removed}
	markManual(&p)
	if !p.NeedsReview {
		t.Fatal("a file removed from Drive still needs a moderator")
	}
}
