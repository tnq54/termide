package ui

import "testing"

func TestTaskFlowCreatesDescriptionAndMovesToAgents(t *testing.T) {
	flow := NewTaskFlow()
	flow.SetDescription("add login endpoint")
	if flow.Description() != "add login endpoint" {
		t.Fatal("description not stored")
	}
	flow.Start()
	if flow.Screen() != ScreenAgents {
		t.Fatalf("screen = %v", flow.Screen())
	}
}
