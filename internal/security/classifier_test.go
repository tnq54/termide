package security

import "testing"

func TestClassifierRequiresApprovalForDangerousCommands(t *testing.T) {
	for _, command := range []string{"rm -rf build", "sudo systemctl restart app", "git push --force", "deploy --production"} {
		if got := Classify(command, "/srv/project"); got != ApprovalRequired { t.Fatalf("%q classified as %s", command, got) }
	}
}

func TestClassifierAllowsReadOnlyDevelopmentCommands(t *testing.T) {
	for _, command := range []string{"git status", "pytest -q", "ruff check ."} {
		if got := Classify(command, "/srv/project"); got != Safe { t.Fatalf("%q classified as %s", command, got) }
	}
}
