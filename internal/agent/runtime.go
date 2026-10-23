package agent

import (
	"context"
	"strings"
	"sync"
)

type Spec struct{ ID, Role, Command, WorkingDir string }
type Event struct{ AgentID, Kind, Output string }
type Runner interface {
	Run(context.Context, Spec, func(Event)) error
}
type Scheduler struct {
	runner      Runner
	maxParallel int
}

func NewScheduler(runner Runner, maxParallel int) *Scheduler {
	if maxParallel < 1 {
		maxParallel = 1
	}
	return &Scheduler{runner: runner, maxParallel: maxParallel}
}
func (s *Scheduler) Run(ctx context.Context, specs []Spec, emit func(Event)) error {
	sem := make(chan struct{}, s.maxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	for _, spec := range specs {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if err := s.runner.Run(ctx, spec, emit); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return first
}
func Redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
