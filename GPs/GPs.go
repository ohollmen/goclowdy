package GPs

import (
	"context"
	"fmt"
	"strings"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/ohollmen/goclowdy/VMs"
	"google.golang.org/api/cloudresourcemanager/v1"
)

type ProjVM struct {
  Project * cloudresourcemanager.Project // Shared
  Vm * computepb.Instance
}
// Parse K=V string to map. Parse by prim/sec. delimiter ?
func KvParse(kv string) map[string]string {
  var m = map[string]string{}
  if kv == "" { return m; }
  arr := strings.Split(kv, "=")
  if len(arr) != 2 { return m; }
  m[arr[0]] = arr[1]
  return m
}

func Map2Query(qmap map[string]string) string {
  var qc []string
  for k, v := range qmap { qc = append(qc, "labels."+k+"="+v) }
  return strings.Join(qc[:], " OR ")
}

// Retrieve all projects (in org)
func ProjectsList(qstr string) []*cloudresourcemanager.Project {
  ctx := context.Background()
	crmService, err := cloudresourcemanager.NewService(ctx)
  if err != nil { fmt.Errorf("error creating CRM: %v", err) ; return nil }
  var plist *cloudresourcemanager.ListProjectsResponse
  // Run SW filtering at client () to avoid Filter() -step (betw. List/Do, Allows to pass a google query filter string like labels.mykey=myval)
  plistcall := crmService.Projects.List()
  if (qstr != "") { plistcall.Filter(qstr) }
  plist, err = plistcall.Do()
  if err   != nil { fmt.Errorf("error: Failed to list projects: %v", err) ; return nil }
  if plist != nil && len(plist.Projects) == 0 { fmt.Println("No projects found");return nil }
  var projlist []*cloudresourcemanager.Project
  for _, project := range plist.Projects {
    //  =>\n%v
    //fmt.Printf("Got project %s\n", project.ProjectId) // , nil
    projlist = append(projlist, project)
  }
  //fmt.Printf("Got %d projects\n", len(projlist))
  return  projlist
}

func ProjectsVMs(Projects []*cloudresourcemanager.Project, vmc VMs.CC) []ProjVM {
  pvms := []ProjVM{}
  for _, project := range Projects {
    vmc.Project = project.ProjectId
    vms := vmc.GetAll()
    if vms == nil { fmt.Println("No VMs from "+vmc.Project) }
    for _, vm := range vms {
      //fmt.Printf("  - VM: %s/%s\n", project.ProjectId, vm.GetName());
      pvms = append(pvms, ProjVM{Project: project, Vm: vm})
    }
  }
  return pvms;
}
