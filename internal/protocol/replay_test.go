package protocol

import (
	"context"
	"testing"

	"termide/internal/domain"
)

type eventSource struct { events []domain.EventEnvelope }
func (s eventSource) EventsAfter(_ context.Context, _ string, sequence uint64) ([]domain.EventEnvelope, error) {
	var out []domain.EventEnvelope
	for _, event := range s.events { if event.Sequence > sequence { out = append(out, event) } }
	return out, nil
}

func TestReplayReturnsEventsAfterClientSequence(t *testing.T) {
	source := eventSource{events: []domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 1, Type: domain.EventOutput}, {WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}}}
	got, err := Replay(context.Background(), source, "w", 1)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Sequence != 2 { t.Fatalf("replay = %#v", got) }
}
