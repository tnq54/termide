package ui

import (
 "context"
 "encoding/json"
 "net"
 "strings"
 "testing"

 tea "github.com/charmbracelet/bubbletea"
 "github.com/charmbracelet/x/ansi"
 "termide/internal/client"
 "termide/internal/domain"
 "termide/internal/server"
 "termide/internal/store"
)

func dashboardFixture(t *testing.T)(Model,*store.Store){
 t.Helper();s,err:=store.Open(t.TempDir()+"/state.db");if err!=nil{t.Fatal(err)}
 workspace:=domain.Workspace{ID:"w",Root:t.TempDir()}
 s.CreateWorkspace(context.Background(),workspace)
 s.CreateTask(context.Background(),domain.Task{ID:"task-a",WorkspaceID:"w",Description:"sửa Python",Status:domain.TaskRunning})
 s.UpsertAgent(context.Background(),domain.Agent{ID:"agent-a",TaskID:"task-a",Role:"coder",Status:domain.AgentRunning})
 payload,_:=json.Marshal(map[string]string{"Output":"hello từ VPS"})
 s.AppendEvent(context.Background(),domain.EventEnvelope{WorkspaceID:"w",TaskID:"task-a",AgentID:"agent-a",Type:domain.EventOutput,Payload:payload})
 left,right:=net.Pipe();done:=make(chan error,1)
 go func(){defer right.Close();done<-server.NewHandler(s).Serve(context.Background(),right,right)}()
 remote:=client.NewRemote(left)
 t.Cleanup(func(){remote.Close();<-done;s.Close()})
 m:=NewRemoteModel(remote,workspace)
 next,load:=m.Update(m.Init()());m=next.(Model)
 if load==nil{t.Fatal("opening workspace did not load task history")}
 next,_=m.Update(load());m=next.(Model)
 next,_=m.Update(tea.WindowSizeMsg{Width:40,Height:18});m=next.(Model)
 return m,s
}

func TestDashboardRestoresTaskHistoryAndRefreshesIncrementalLogs(t *testing.T){
 m,s:=dashboardFixture(t)
 if len(m.dashboard.tasks)!=1||!strings.Contains(m.View(),"sửa Python"){t.Fatal(m.View())}
 m=applyCommand(t,m,tea.KeyMsg{Type:tea.KeyEnter})
 if m.Panel!=PanelAgents||m.dashboard.snapshot==nil||!strings.Contains(m.View(),"hello từ VPS"){t.Fatal(m.View())}
 payload,_:=json.Marshal(map[string]string{"Output":"finished"})
 s.AppendEvent(context.Background(),domain.EventEnvelope{WorkspaceID:"w",TaskID:"task-a",AgentID:"agent-a",Type:domain.EventOutput,Payload:payload})
 s.UpdateTaskStatus(context.Background(),"task-a",domain.TaskCompleted)
 m=applyCommand(t,m,key("r"))
 if m.dashboard.snapshot.Task.Status!=domain.TaskCompleted{t.Fatal("status not refreshed")}
 if len(m.dashboard.logs)!=2{t.Fatal("missing or duplicate log lines")}
 m=applyCommand(t,m,key("r"));if len(m.dashboard.logs)!=2{t.Fatal("logs duplicated")}
}

func TestDashboardSmallViewportAndUntrustedOutput(t *testing.T){
 m,s:=dashboardFixture(t)
 payload,_:=json.Marshal(map[string]string{"Output":"\x1b[2Jmalicious\rcontent"})
 s.AppendEvent(context.Background(),domain.EventEnvelope{WorkspaceID:"w",TaskID:"task-a",AgentID:"agent-a",Type:domain.EventOutput,Payload:payload})
 m=applyCommand(t,m,tea.KeyMsg{Type:tea.KeyEnter})
 for _,size:=range [][2]int{{40,12},{20,8},{1,1}}{
  next,_:=m.Update(tea.WindowSizeMsg{Width:size[0],Height:size[1]});m=next.(Model)
  view:=m.View();if strings.Contains(view,"\x1b[2J"){t.Fatal("terminal control sequence leaked")}
  lines:=strings.Split(view,"\n");if len(lines)>size[1]{t.Fatal("height overflow")}
  for _,line:=range lines{if ansi.StringWidth(line)>size[0]{t.Fatal("width overflow")}}
 }
}
