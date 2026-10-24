package ui

import (
	"testing"

	"termide/internal/domain"
)

func TestApprovalPanelRendersCommandAndReason(t *testing.T) {
	text := RenderApproval(domain.ApprovalRequest{ID: "ap-1", Command: "rm -rf build", Reason: "agent cleanup"})
	if text == "" || !contains(text, "rm -rf build") || !contains(text, "agent cleanup") {
		t.Fatalf("rendered = %q", text)
	}
}
func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
