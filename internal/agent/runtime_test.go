package agent

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu sync.Mutex
	started []string
}
func (f *fakeRunner) Run(ctx context.Context, spec Spec, emit func(Event)) error {
	f.mu.Lock(); f.started = append(f.started, spec.ID); f.mu.Unlock()
	emit(Event{AgentID: spec.ID, Kind: "started"})
	select { case <-time.After(10 * time.Millisecond): case <-ctx.Done(): return ctx.Err() }
	emit(Event{AgentID: spec.ID, Kind: "completed"})
	return nil
}

func TestSchedulerRunsIndependentAgentsInParallel(t *testing.T) {
	runner := &fakeRunner{}
	s := NewScheduler(runner, 2)
	plan := []Spec{{ID: "a1", Role: "coder"}, {ID: "a2", Role: "tester"}}
	var mu sync.Mutex
	var events []Event
	err := s.Run(context.Background(), plan, func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() })
	if err != nil { t.Fatal(err) }
	if len(events) != 4 { t.Fatalf("events = %#v", events) }
}

func TestRedactSecrets(t *testing.T) {
	got := Redact("token=secret-value", []string{"secret-value"})
	if got != "token=[REDACTED]" { t.Fatalf("got %q", got) }
}
