package server

import (
 "context"
 "encoding/json"
 "termide/internal/protocol"
)

func(h *Handler)handleDashboard(ctx context.Context,message protocol.Message)(protocol.Message,error){
 if _,err:=h.store.GetWorkspace(ctx,message.WorkspaceID);err!=nil{return protocol.Message{},err}
 response:=protocol.Message{WorkspaceID:message.WorkspaceID,TaskID:message.TaskID}
 switch message.Type{
 case protocol.MessageListTasks:
  var request protocol.TaskPageRequest
  if len(message.Payload)>0{if err:=json.Unmarshal(message.Payload,&request);err!=nil{return response,err}}
  page,err:=h.store.TaskPage(ctx,message.WorkspaceID,request.Before,50);if err!=nil{return response,err}
  response.Type=protocol.MessageTaskPage;response.Payload,err=json.Marshal(page);return response,err
 case protocol.MessageTaskSnapshot:
  snapshot,err:=h.store.TaskSnapshot(ctx,message.WorkspaceID,message.TaskID,message.AfterSequence,50);if err!=nil{return response,err}
  response.Type=protocol.MessageTaskDetail;response.Payload,err=json.Marshal(snapshot);return response,err
 }
 panic("unreachable dashboard message")
}
