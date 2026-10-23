package server

import (
 "context"
 "encoding/json"
 "testing"
 "termide/internal/domain"
 "termide/internal/protocol"
 "termide/internal/store"
)

func TestDashboardProtocolScopesTasksAndReturnsSnapshot(t *testing.T){
 ctx:=context.Background();s,err:=store.Open(t.TempDir()+"/state.db");if err!=nil{t.Fatal(err)};defer s.Close()
 for _,id:=range []string{"w","other"}{s.CreateWorkspace(ctx,domain.Workspace{ID:id})}
 s.CreateTask(ctx,domain.Task{ID:"t",WorkspaceID:"w",Description:"real task",Status:domain.TaskQueued})
 h:=NewHandler(s)
 page,err:=h.Handle(ctx,protocol.Message{Type:protocol.MessageListTasks,WorkspaceID:"w"});if err!=nil{t.Fatal(err)}
 var decoded domain.TaskPage
 if err:=json.Unmarshal(page.Payload,&decoded);err!=nil{t.Fatal(err)}
 if page.Type!=protocol.MessageTaskPage||len(decoded.Tasks)!=1||decoded.Tasks[0].ID!="t"{t.Fatal(page)}
 detail,err:=h.Handle(ctx,protocol.Message{Type:protocol.MessageTaskSnapshot,WorkspaceID:"w",TaskID:"t"});if err!=nil{t.Fatal(err)}
 var snapshot domain.TaskSnapshot;json.Unmarshal(detail.Payload,&snapshot)
 if snapshot.Task.Description!="real task"{t.Fatal(snapshot)}
 if _,err:=h.Handle(ctx,protocol.Message{Type:protocol.MessageTaskSnapshot,WorkspaceID:"other",TaskID:"t"});err==nil{t.Fatal("cross-workspace detail")}
 if _,err:=h.Handle(ctx,protocol.Message{Type:protocol.MessageListTasks,WorkspaceID:"missing"});err==nil{t.Fatal("missing workspace accepted")}
}
