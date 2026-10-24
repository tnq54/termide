package ui

type Screen string

const (
	ScreenTask   Screen = "task"
	ScreenAgents Screen = "agents"
)

type TaskFlow struct {
	description string
	screen      Screen
}

func NewTaskFlow() *TaskFlow                          { return &TaskFlow{screen: ScreenTask} }
func (f *TaskFlow) SetDescription(description string) { f.description = description }
func (f *TaskFlow) Description() string               { return f.description }
func (f *TaskFlow) Start() {
	if f.description != "" {
		f.screen = ScreenAgents
	}
}
func (f *TaskFlow) Screen() Screen { return f.screen }
