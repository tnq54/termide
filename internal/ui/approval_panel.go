package ui

import (
	"fmt"
	"termide/internal/domain"
)

func RenderApproval(request domain.ApprovalRequest) string {
	return fmt.Sprintf("Approval %s\nCommand: %s\nReason: %s\nWorking directory: %s\n[a]pprove [d]eny", request.ID, request.Command, request.Reason, request.WorkingDir)
}
