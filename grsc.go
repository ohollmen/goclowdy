// Deletion Policy parametes
//                     (1 Y+1 w)            (1 w)
// Abs.Age            KeepMaxH             KeepMinH         Now
// <---------------------|--------------------|---------------|
//  keep none/del all    |<-- keep 1/week  -->|  keep all     |
// ## Building
// ```
// go build grsc.go
// ```
// Resources:
// - https://ueokande.github.io/go-slice-tricks/
// - 
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
  VMs "github.com/ohollmen/goclowdy/VMs"
  MIs "github.com/ohollmen/goclowdy/MIs"
  //"github.com/ohollmen/goclowdy"
  //"goclowdy/vm/VMs"
  //"goclowdy/mi/MIs"
  //MIs "goclowdy/mi"
  //VMs "goclowdy/vm" 
  //"goclowdy/VMs"
  //"goclowdy/MIs"
  "path" // Base  
  "google.golang.org/api/iam/v1"
  // NEW
  //"regexp" // responsibility moved to MIs
  "sync" // go get -u golang.org/x/sync
)

var verdict = [...]string{"KEEP to be safe", "KEEP NEW (< KeepMinH)", "KEEP (MID-TERM, WEEKLY)", "DELETE (MID-TERM)", "DELETE OLD (> KeepMaxH)", "KEEP-NON-STD-NAME"}
var envcfgkeys = [...]string{"GCP_PROJECT","GOOGLE_APPLICATION_CREDENTIALS","MI_DELETE_EXEC","MI_STDNAME"}
// Machine image mini-info. Allow deletion to utilize this (for reporting output). Add creation time ?
type MIMI struct {
  miname string
  class int
}
func main() {
  //ctx := context.Background()
  if len(os.Args) < 2 { fmt.Println("Pass one of subcommands: vmlist,midel,keylist,env"); return }
  //if () {}
  pname := os.Getenv("GCP_PROJECT")
  if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  if os.Args[1] == "vmlist" {
    vm_ls(pname)
  } else if os.Args[1] == "midel" {
    mi_del(pname)
  } else if os.Args[1] == "keylist" {
    key_list(pname)
  } else if os.Args[1] == "env" {
    fmt.Println("# The environment config:")
    for _, evar := range envcfgkeys { 
      fmt.Println("export "+ evar+ "="+ os.Getenv(evar)+ "") 
    } 
  } else if os.Args[1] == "subarr" {
    chunks := chunk(names2, 3);
    //chunk(names2, 4);
    //chunk(names2, 5);
    //chunk(names2, 20);
    for i, chunk := range chunks {
      fmt.Printf("Chunk %d: %+v\n", i, chunk);
      var wg sync.WaitGroup
      for _, item := range chunk{
        wg.Add(1)
        go hello(item, &wg)
      }
      wg.Wait()
    }
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

// Delet machine images per given config policy
func mi_del(pname string) {
  ctx := context.Background()
  midel := os.Getenv("MI_DELETE_EXEC")
  // https://pkg.go.dev/regexp
  //var stdnamere * regexp.Regexp // Regexp
  //var err error
  // E.g. "^\\w+-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}" (in Go runtime)
  // E.g. "^[a-z0-9-]+?-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}" // No need for 1) \\ before [..-] 2) \ before [ / ]
  /*
  if (os.Getenv("MI_STDNAME") != "") {
    stdnamere, err = regexp.Compile(os.Getenv("MI_STDNAME")) // (*Regexp, error) // Also MustCompile
    if err != nil { fmt.Println("Cannot compile STD name RegExp"); return }
    //stdm := stdnamere.MatchString( "myhost-00-00-00-00-1900-01-01" ); // Also reg.MatchString() reg.FindString() []byte()
    //if !stdm { fmt.Println("STD Name re not matching "); return }
  }
  */
  //return
  // 168 h = 1 = week, (24 * (365 + 7)) hours = 1 year,  weekday 5 = Friday (wdays: 0=Sun... 6=Sat)
  mic := MIs.CC{Project: pname,  WD_keep: 5, KeepMinH: 168,  KeepMaxH: (24 * (365 + 7)), TZn: "Europe/London"} // tnow: tnow, tloc: loc
  rc := mic.Init()
  if rc != 0 { fmt.Printf("Machine image module init failed: %d\n", rc); return }
  if midel != "" { mic.DelOK = true; } // Non-empty => DELETE
  var maxr uint32 = 20
  if mic.Project == "" { fmt.Println("No Project passed"); return }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  //fmt.Println("Search MI from: "+cfg.Project+", parse by: "+time.RFC3339)
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No mis from "+mic.Project); }
  // https://code.googlesource.com/gocloud/+/refs/tags/v0.101.1/compute/apiv1/machine_images_client.go
  //var delarr []*computepb.MachineImage // var item *computepb.MachineImage
  var delarr []MIMI
  // Iterate MIs, check for need to del
  totcnt := 0; todel := 0;
  for {
    //fmt.Println("Next ...");
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break }
    if mi == nil {  fmt.Println("No mi. check (actual) creds, project etc."); break }
    
    fmt.Println("MI:"+mi.GetName()+" (Created: "+mi.GetCreationTimestamp()+")")
    // OLD: stdnamere
    //if (mic.NameRE != nil) && (!mic.NameRE.MatchString( mi.GetName() )) {
    //  fmt.Printf("  - NON-STD-NAME: %s\n", mi.GetName())
    //  continue;
    //}
    var cl int = mic.Classify(mi)
    fmt.Println(verdict[cl])
    if MIs.ToBeDeleted(cl) {
      todel++
      fmt.Printf("DELETE %s\n", mi.GetName()) // Also in DRYRUN
      mimi := MIMI{miname: mi.GetName(), class: cl}
      // Store MI to list
      //delarr = append(delarr, mi)
      delarr = append(delarr, mimi)
    } else {
      fmt.Printf("KEEP %s\n", mi.GetName())
    }
    fmt.Printf("============\n")
    totcnt++
  }
  if !mic.DelOK { fmt.Printf("# Dry-run mode, DelOK = %t (%d to delete)\n", mic.DelOK, todel); return; }
  // Delete Here ?
  for _, mi := range delarr { // i
    fmt.Printf("%s\n", mi.miname, verdict[mi.class]) // mi.GetName()
    if mic.DelOK {
      //OLD:rc := mic_delete_mi(&mic, mi.GetName());
      rc := mic_delete_mi(&mic, mi.miname);
      if rc != 0 { }
    }
  }
}

func mic_delete_mi(mic * MIs.CC, miname string) int { // mi *computepb.MachineImage
  err := mic.Delete(miname) // mi.GetName()
  if err != nil {
   fmt.Printf("Error Deleting MI: %s\n", miname) //  mi.GetName()
   return 1
  } else {
   fmt.Printf("Deleted %s\n", miname) //  mi.GetName()
   return 0
  }
  //fmt.Printf("Should have deleted %s. Set DelOK (MI_DELETE_EXEC) to actually delete.\n", mi.GetName())
  return 0
}

func key_list(pname string) {
  ctx := context.Background()
  iamService, err := iam.NewService(ctx)
  if err != nil { fmt.Println("No Service"); return }
  acct := os.Getenv("GCP_SA")
  pname = os.Getenv("GCP_SA_PROJECT") // override
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
}
