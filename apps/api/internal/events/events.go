// Package events appends to the per-group event log and fans events out to
// in-process subscribers (activity feed, notifications, realtime hub). The
// log is the single source for offline sync and realtime; see docs/PLAN.md §7.
package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

// Subscriber receives committed events. It must not block.
type Subscriber func(e domain.Event)

// Bus dispatches committed events to subscribers in the same process.
type Bus struct {
	mu   sync.RWMutex
	subs []Subscriber
}

// NewBus creates an empty bus.
func NewBus() *Bus { return &Bus{} }

// Subscribe registers a subscriber.
func (b *Bus) Subscribe(s Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, s)
}

func (b *Bus) publish(e domain.Event) {
	b.mu.RLock()
	subs := append([]Subscriber(nil), b.subs...)
	b.mu.RUnlock()
	for _, s := range subs {
		s(e)
	}
}

// Publisher assigns sequence numbers and persists events atomically with the
// business change that produced them.
type Publisher struct {
	groups domain.GroupRepo
	events domain.EventRepo
	tx     domain.TxManager
	bus    *Bus
	log    *slog.Logger
}

// NewPublisher wires a publisher.
func NewPublisher(groups domain.GroupRepo, events domain.EventRepo, tx domain.TxManager, bus *Bus, log *slog.Logger) *Publisher {
	return &Publisher{groups: groups, events: events, tx: tx, bus: bus, log: log}
}

// Publish stores e with the next group sequence number. When called inside a
// transaction it joins it and subscribers are notified after commit; otherwise
// it runs in its own transaction.
func (p *Publisher) Publish(ctx context.Context, e domain.Event) (*domain.Event, error) {
	var saved *domain.Event
	err := p.tx.RunInTx(ctx, func(ctx context.Context) error {
		seq, err := p.groups.NextSeq(ctx, e.GroupID)
		if err != nil {
			return fmt.Errorf("next seq: %w", err)
		}
		e.ID = ids.New()
		e.Seq = seq
		e.CreatedAt = time.Now().UTC()
		saved, err = p.events.Insert(ctx, e)
		if err != nil {
			return err
		}
		ev := *saved
		p.tx.AfterCommit(ctx, func() {
			defer func() {
				if r := recover(); r != nil {
					p.log.Error("event subscriber panicked", "kind", ev.Kind, "panic", r)
				}
			}()
			p.bus.publish(ev)
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// Emit is a convenience wrapper: builds and publishes an event, logging
// (rather than failing) when the payload cannot be marshalled.
func (p *Publisher) Emit(ctx context.Context, e domain.Event, err error) error {
	if err != nil {
		return fmt.Errorf("build event %s: %w", e.Kind, err)
	}
	_, err = p.Publish(ctx, e)
	return err
}
