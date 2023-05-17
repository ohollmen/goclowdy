package main

// go.formatOnSave
// editor.formatOnSave
// go build grsc.go
import (
  "context"
  "fmt"
  "os" // Args  
  "google.golang.org/api/iterator"  
  //compute "cloud.google.com/go/compute/apiv1" // Used only in lower levels
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  //goc "VMs"
  //macv "MIs"
  //VMs "github.com/ohollmen/goclowdy/VMs"
  //MIs "github.com/ohollmen/goclowdy/MIs"
  //"github.com/ohollmen/goclowdy"
  //"goclowdy/vm/VMs"
  //"goclowdy/mi/MIs"
  MIs "goclowdy/mi"
  VMs "goclowdy/vm" 
  //"goclowdy/VMs"
  //"goclowdy/MIs"
  "path" // Base  
  "google.golang.org/api/iam/v1"
)

var verdict = [...]string{"KEEP to be safe", "KEEP Recent (<1W)", "KEEP (one-per-week)", "DELETE (>1W)", "DELETE (> 1Y)"}

func main() {
  //ctx := context.Background()
  if len(os.Args) < 2 { fmt.Println("Pass one of subcommands: vmlist,midel"); return }
  //if () {}
  pname := os.Getenv("GCP_PROJECT")
  if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  if os.Args[1] == "vmlist" {
    vm_ls(pname)
  } else if os.Args[1] == "midel" {
    mi_del(pname)
  } else if os.Args[1] == "keylist" {
    ctx := context.Background()
    iamService, err := iam.NewService(ctx)
    if err != nil { fmt.Println("No Service"); return }
    acct := os.Getenv("GCP_SA")
    pname := os.Getenv("GCP_SA_PROJECT")
    if acct == "" { fmt.Println("No GCP_SA"); return }
    if pname == "" { fmt.Println("No GCP_SA_PROJECT"); return }
    sapath := fmt.Sprintf( "projects/%s/serviceAccounts/%s", pname,  acct)
    resp, err := iamService.Projects.ServiceAccounts.Keys.List(sapath).Context(ctx).Do()
    if err != nil { fmt.Println("No Keys %v", err); return }
    //fmt.Println("Got:", resp) // iam.ListServiceAccountKeysResponse
    fmt.Printf("%T\n", resp) // import "reflect" fmt.Println(reflect.TypeOf(tst))
    for _, key := range resp.Keys {
      fmt.Printf("%T\n", key) // iam.ServiceAccountKey
      fmt.Printf("%v Exp.: %s\n", path.Base(key.Name), key.ValidBeforeTime)
    }
    // Also: SignJwtRequest, but: https://cloud.google.com/iam/docs/migrating-to-credentials-api
  } else { fmt.Println("Pass one of subcommands: vmlist,milist,keylist"); return }
  return
}

func vm_ls(pname string) {
    //ctx := context.Background()
    // test overlapping sysm (old: vs)
    vmc := VMs.CC{Project: pname}
    vmc.Init()
    all := vmc.GetAll()
    //fmt.Println(all)
    icnt := len(all)
    if icnt == 0 { fmt.Println("No VMs found"); return }
    fmt.Printf("Got %v Initial Instances (Filtering ...)\n", icnt)
    // Test for daily MI. This is now embedded to mic.CreateFrom() logic.
    mic := MIs.CC{Project: pname,  WD_keep: 5, KeepMinH: 168,  TZn: "Europe/London", Debug: true} // tnow: tnow, tloc: loc
    mic.Init()
    for _, it := range all{ // Instance
      fmt.Println("vname:"+it.GetName())
      in := MIs.StdName(it.GetName())
      fmt.Println("STD Name:", in)
      mi := mic.GetOne(in)
      if mi != nil  {
        fmt.Println("Found image: ", mi.GetName())
        //mic.Delete(mi)
      } else { fmt.Println("No (std) image found for : ", it.GetName()) }
    }
    return
}
func mi_del(pname string) {
  ctx := context.Background()
  midel := os.Getenv("MI_DELETE_EXEC")
  mic := MIs.CC{Project: pname,  WD_keep: 5, KeepMinH: 168,  KeepMaxH: (24 * (365 + 7)), TZn: "Europe/London"} // tnow: tnow, tloc: loc
  mic.Init()
  if midel != "" { mic.DelOK = true; }
  var maxr uint32 = 20
  if mic.Project == "" { fmt.Println("No Project passed"); return }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  //fmt.Println("Search MI from: "+cfg.Project+", parse by: "+time.RFC3339)
  it := mic.Client().List(ctx, req)  
  for {
    mi, err := it.Next()
    if err == iterator.Done { fmt.Println("Iter done"); break }
    if mi == nil {  fmt.Println("No mi"); break }
    
    fmt.Println("MI:"+mi.GetName()+" "+mi.GetCreationTimestamp())
    //var cl int = MIs.Classify(mi, &mic)
    var cl int = mic.Classify(mi)
    fmt.Println(verdict[cl])
    if MIs.ToBeDeleted(cl) {
      if mic.DelOK {

        //err := mic.Delete(mi)
        // if err != nil {
        //  fmt.Printf("Error Deleting MI: %s.", mi.GetName())
        //} else {
        fmt.Printf("Deleted %s.", mi.GetName())
        //}
      } else {
        fmt.Printf("Should have deleted %s. Set DelOK (MI_DELETE_EXEC) to actually delete.\n", mi.GetName())
      }
    }
  }
}
