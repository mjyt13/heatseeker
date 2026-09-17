package jobs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"

	"heatseeker/api/internal/domain"
)

func TestScheduleSpecsParse(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	seen := map[string]bool{}
	for _, e := range Schedule() {
		if _, err := parser.Parse(e.Spec); err != nil {
			t.Errorf("%s: bad spec %q: %v", e.Task.Type(), e.Spec, err)
		}
		if seen[e.Task.Type()] {
			t.Errorf("%s scheduled twice", e.Task.Type())
		}
		seen[e.Task.Type()] = true
	}
	if !seen[domain.JobDriveSyncDue] {
		t.Error("drive sync is not scheduled")
	}
}

func TestPermanent(t *testing.T) {
	if permanent(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if err := permanent(fmt.Errorf("x: %w", domain.ErrUnavailable)); !errors.Is(err, asynq.SkipRetry) || !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unavailable must skip retries and keep its cause: %v", err)
	}
	if err := permanent(errors.New("network")); errors.Is(err, asynq.SkipRetry) {
		t.Fatal("transient errors must be retried")
	}
}

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
