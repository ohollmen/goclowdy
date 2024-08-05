// Collect foundational elems Org, Folders, Projects under **single** Org.
// Note:
//   - Project and Folder loading could be merged into one with minor if-elsing
//     (Possibly have constructor-callbacks to create OrgEnt for different types)
//
// May require gcloud auth application-default login (?) - although does not seem so.
// Also try:
// gcloud organizations list (to see your orgid)
// gcloud projects list --organization=
// gcloud projects list --filter 'parent.id=id-organization123456 AND parent.type=organization'
// gcloud resource-manager folders list --organization ...
// gcloud beta asset search-all-resources --asset-types=cloudresourcemanager.googleapis.com/Project --scope=organizations/12345
package OrgTree

import (
	"context"
	"fmt"
	"strings"
	"time"

	//"io"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/iterator"
)

type orgnodecb func(oe * OrgEnt, userdata * interface{}) error // , if Interface (userdata)

type OrgEnt struct {
  Name  string `json:"name"`
  Id    string `json:"id"`    // Number on orgs and folders, letter-str on projects	
  Etype string `json:"etype"` // "organization", "folder", "project"
  Children []*OrgEnt `json:"children"`
  Userdata * interface{}
}
// OrgLoader
type OrgLoader struct {
  f * resourcemanager.FoldersClient
  p * resourcemanager.ProjectsClient
  //ctx * context
  fcnt int
  pcnt int
  Debug bool
  delay time.Duration
}

// Serializer ("stringer") for OrgEnt
func (oe OrgEnt) String() string {
  return fmt.Sprintf("{%v %v %v}", oe.Name, oe.Id, oe.Etype)
}
func (oload * OrgLoader) LoadInit() {
  ctx := context.Background()
  var err error
  oload.f, err = resourcemanager.NewFoldersClient(ctx)
  if err != nil { fmt.Printf("Error: creating NewFoldersClient: %s\n", err); return; }
  oload.p, err = resourcemanager.NewProjectsClient(ctx)
  if err != nil { fmt.Printf("Error: creating NewProjectsClient: %s\n", err); return; }
  oload.delay = time.Millisecond * 20
  return
}
// Load org tree by Creating Stub-Org by orgid, orgname
func NewOrgTree(orgid string, orgname string) *OrgEnt{
  if orgid == "" { return nil; } // Also Error ?
  root := &OrgEnt{ Name: orgname, Id: orgid, Etype: "organization" };
  if root.Name == "" { root.Name = "My Organization" } // Force *some* naming
  if root == nil { return nil; }
  return root
}
// Load Org Tree by traversing / searching the projects and folders.
// Make all ents of orgtree to be of (homogenous) type OrgEnt.
func (oload * OrgLoader) LoadOrgTree(oe * OrgEnt) {
  if oe == nil  { return; }
  projects := oload.Projects(oe) // always slice
  // Projects are terminal leaves - no children / no need to traverse
  for _, p := range projects { oe.Children = append(oe.Children, p) }
  folders := oload.Folders(oe);
  if len(folders) < 0 { return; }
  for _, f := range folders {
    oe.Children = append(oe.Children, f)
    oload.LoadOrgTree(f) // recurse (into folder)
  }
}
// Load Folders under Current OrgEnt
func (oload * OrgLoader) Folders( oe * OrgEnt) []*OrgEnt {
  ctx := context.Background()
  children := []*OrgEnt{}
  filter := fmt.Sprintf("parent:%ss/%s", oe.Etype, oe.Id)
  fmt.Printf("Search folders by: %s\n", filter);
  r := &resourcemanagerpb.SearchFoldersRequest{ Query: filter,}
  it := oload.f.SearchFolders(ctx, r)
  for {
    f, err := it.Next()
    if err == iterator.Done { fmt.Printf("Done Iter\n");break }
    if err != nil { fmt.Printf("Error: Failed s-by-f: (%s): %s\n", filter, err); break }
    fmt.Printf("Got folder: Name: %s, IdRAW: %s - add ...\n", f.DisplayName, f.Name);
    oe := &OrgEnt{ Name: f.DisplayName, Id: strings.Split(f.Name, "/")[1], Etype: "folder",}
    children = append(children, oe)
    time.Sleep(oload.delay)
  }
  //ccnt := len(children);
  //if ccnt > 0 { oe.Children = children; } // let caller handle this
  if oload.Debug { fmt.Printf("children(folders): %+v\n", children); }
  return children
}
// Load Projects under current OrgEnt
func (oload * OrgLoader) Projects( oe * OrgEnt ) []*OrgEnt {
  ctx := context.Background()
  children := []*OrgEnt{}
  filter := fmt.Sprintf("parent:%ss/%s", oe.Etype, oe.Id)
  fmt.Printf("Search projects by: %s\n", filter);
  rp := &resourcemanagerpb.SearchProjectsRequest{ Query: filter, }
  it := oload.p.SearchProjects(ctx, rp)
  for {
    p, err := it.Next()
    if err == iterator.Done { break }
    if err != nil { fmt.Printf("Error: s-by-f (%s): %s\n", filter, err); break }
    oe := &OrgEnt{ Name: p.DisplayName, Id: p.ProjectId, Etype: "project",} 
    children = append(children, oe)
    time.Sleep(oload.delay)
  }
  if oload.Debug { fmt.Printf("children(proj): %+v\n", children); }
  return children
}
// Process OrgEnt Tree with a callback (and TODO: user data => Interface).
// Callback can make a processing-related filtering decisions internally e.g. by only processing certain types:
//     if (oe.Etype == "org") || (oe.Etype == "folder") { return; } // SKIP
//     // ... continue processing
// TODO: Add (generic) userdata (Interface ?) to orgnodecb
func (oe * OrgEnt) Process(cb orgnodecb, userdata * interface{}) {
  err := cb(oe, userdata)
  if err != nil { fmt.Printf("Error processing node !"); return; } // Terminate traversal of this tree branch !
  for _, oec := range oe.Children {
    //err :=
    oec.Process(cb, userdata)
    //if err != nil { fmt.Printf("Error processing child-node !"); }
  }
  //if err != nil { return err; }
  //return nil
}
// Test orgnodecb
// Try out by root.Process(OrgTree.Dumpent, nil) // userdata = nil
func Dumpent(oe * OrgEnt, dummy * interface{}) error {
  fmt.Printf("OE-DUMP: %s\n", oe);
  return nil
}
/*
func main() {
  
  oname := "my.org";
  oid := "007"
  if os.Getenv("ORGID") != "" { oid = os.Getenv("ORGID"); }
  var oload = OrgLoader{};
  oload.LoadInit()
  oload.Debug = true
  root := NewOrgTree(oid, oname);
  fmt.Printf("Constructed Org, but skipping traverse\n"); return;
  oload.LoadOrgTree(root);
  dump, err := json.MarshalIndent(root, "", "  ")
  if err != nil { fmt.Printf("Error serializing to JSON: %s\n", err); }
  fmt.Printf("Done main: %s\n", dump);

}
*/
