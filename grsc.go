// Deletion Policy parametes
//
//	(1 Y+1 w)            (1 w)
//
// Abs.Age            KeepMaxH             KeepMinH         Now
// <---------------------|--------------------|---------------|
//
//	keep none/del all    |<-- keep 1/week  -->|  keep all     |
//
// ## Building
// ```
// go build grsc.go
// ```
// Resources:
// - https://ueokande.github.io/go-slice-tricks/
// - Go gotchas on slices (pointers to item, etc)
//   - https://medium.com/@betable/3-go-gotchas-590b8c014e0a
//   - https://dev.to/kkentzo/the-golang-for-loop-gotcha-1n35
package main

// go.formatOnSave
// editor.formatOnSave
// go build grsc.go
import (
	"context"
	"fmt"
	"os" // Args
	"time"

	"google.golang.org/api/iterator"

	//compute "cloud.google.com/go/compute/apiv1" // Used only in lower levels
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	//goc "VMs"
	//macv "MIs"
	MIs "github.com/ohollmen/goclowdy/MIs"
	VMs "github.com/ohollmen/goclowdy/VMs"

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
	"encoding/json"
	"sync" // go get -u golang.org/x/sync

	"math/rand"

	"flag"

	"github.com/ohollmen/goclowdy/workerpool"
)

var verdict = [...]string{"KEEP to be safe", "KEEP NEW (< KeepMinH)", "KEEP (MID-TERM, WEEKLY)", "DELETE (MID-TERM)", "DELETE OLD (> KeepMaxH)", "KEEP-NON-STD-NAME"}
var envcfgkeys = [...]string{"GCP_PROJECT","GOOGLE_APPLICATION_CREDENTIALS","MI_DELETE_EXEC","MI_STDNAME", "MI_CHUNK_DEL_SIZE"}
var wdnames = []string{"SUN","MON","TUE","WED","THU","FRI","SAT"}
// Default MI client config
// 168 h = 1 = week, (24 * (365 + 7)) hours = 1 year,  weekday 5 = Friday (wdays: 0=Sun... 6=Sat)
var mic  MIs.CC = MIs.CC{Project: "",  WD_keep: 5, KeepMinH: 168,  KeepMaxH: (24 * (365 + 7)), TZn: "Europe/London"} // tnow: tnow, tloc: loc
// Machine image mini-info. Allow deletion to utilize this (for reporting output). Add creation time ?
type MIMI struct {
  miname string
  class int
}
func main() {
  //ctx := context.Background()
  //flag.StringVar(&(mic.Project), "project", "", "GCP Cloud project (string) id")
  //fooCmd := flag.NewFlagSet("foo", 0) // flag.ExitOnError
  //if fooCmd != nil { return; }
  var project string
  flag.StringVar(&project, "project", "NoN", "GCP Cloud project (string) id")
  flag.Parse() // os.Args[2:] From Args[2] onwards
  fmt.Printf("XXXX Project=%s\n", project)
  subcmds := "vm_mi_list,midel,keylist,env"
  if len(os.Args) < 2 { fmt.Println("Pass one of subcommands: "+subcmds); return }
  //if () {}
  //pname := os.Getenv("GCP_PROJECT")
  //if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  //if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  config_load("", &mic);
  
  //return
  if os.Args[1] == "vm_mi_list" {
    vm_ls()
  } else if os.Args[1] == "midel" {
    mi_del()
  } else if os.Args[1] == "keylist" {
    key_list()
  } else if os.Args[1] == "env" {
    fmt.Println("# The environment config:")
    for _, evar := range envcfgkeys { fmt.Println("export "+ evar+ "='"+ os.Getenv(evar)+ "'") }
    jb, _ := json.MarshalIndent(&mic, "", "  ")
    fmt.Printf("# Config as JSON (After config load ONLY):\n%s\n\n", jb)
    mic.Init();
    for _, evar := range envcfgkeys { fmt.Println("export "+ evar+ "='"+ os.Getenv(evar)+ "'") }
    //flag.Parse()
    //fmt.Printf("XXXX Project=%s\n", project)
    jb, _ = json.MarshalIndent(&mic, "", "  ")
    fmt.Printf("# Config as JSON (After config load and Init()):\n%s\n\n", jb)
    if mic.NameRE != nil { fmt.Printf(" MI-RE:\n# - As Str:  %s\n# - From RE: %s\n", mic.NameREStr, mic.NameRE); }
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
  } else if os.Args[1] == "milist" {
    mi_list()
  } else { fmt.Println("Pass one of subcommands: "+subcmds); return }
  return
}

func vm_ls() { // pname string
    //ctx := context.Background()
    // test overlapping sysm (old: vs)
    vmc := VMs.CC{Project: mic.Project}
    vmc.Init()
    all := vmc.GetAll()
    //fmt.Println(all)
    icnt := len(all)
    if icnt == 0 { fmt.Println("No VMs found"); return }
    fmt.Printf("Got %v Initial Instances (Filtering ...)\n", icnt)
    // Test for daily MI. This is now embedded to mic.CreateFrom() logic.
    //mic := MIs.CC{Project: pname,  WD_keep: 5, KeepMinH: 168,  TZn: "Europe/London", Debug: true} // tnow: tnow, tloc: loc
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
func mi_list() {
  ctx := context.Background()
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Clinet Init() failed: %d (%+v)\n", rc, &mic);  }
  var maxr uint32 = 500 // 20
  if mic.Project == "" { fmt.Println("No Project passed"); return }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  if req == nil { return; }
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No mi:s from "+mic.Project); }
  totcnt := 0
  for {
    //fmt.Println("Next ...");
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break }
    if mi == nil {  fmt.Println("No mi. check (actual) creds, project etc."); break }
    // NOTE: We are not deleting here, only classifying (w. interest in KEEP_WD, DEL_1W)
    var cl int = mic.Classify(mi)
    //if verbose { fmt.Println(verdict[cl]) }
    if (cl == MIs.KEEP_WD) || (cl == MIs.DEL_1W) {
      t, _ := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), mic.Tloc) // Def. UTC
      wd := int(t.Weekday());
      fmt.Printf("%s %s %s %s\n", mi.GetName(), mi.GetCreationTimestamp(), verdict[cl], wdnames[wd]) // 
      if mic.Debug { fmt.Printf("%s %s (%d)\n", t.UTC(), t.UTC().Weekday(), int(t.UTC().Weekday()) ); } // DEBUG UTC()
    }
  }
}
// Delete machine images per given config policy
func mi_del() { // pname string
  ctx := context.Background()
  //config_load("", &mic); // Already on top
  
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Clinet Init() failed: %d (%+v)\n", rc, &mic);  }
  fmt.Printf("Config (after init): %+v\n", &mic);
  if rc != 0 { fmt.Printf("Machine image module init failed: %d\n", rc); return }
  miclstat := map[int]int{0: 0, 1:0, 2:0, 3:0, 4:0, 5:0}
  //TEST: miclstat_out(mic, miclstat); return;
  var maxr uint32 = 500 // 20
  if mic.Project == "" { fmt.Println("No Project passed"); return }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  //fmt.Println("Search MI from: "+cfg.Project+", parse by: "+time.RFC3339)
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No mi:s from "+mic.Project); }
  // https://code.googlesource.com/gocloud/+/refs/tags/v0.101.1/compute/apiv1/machine_images_client.go
  //var delarr []*computepb.MachineImage // var item *computepb.MachineImage
  var delarr []MIMI
  // Iterate MIs, check for need to del
  totcnt := 0; todel := 0;
  verbose := true
  // Classification stats. Note: no wrapping make() needed w. element initialization
  
  for {
    //fmt.Println("Next ...");
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break }
    if mi == nil {  fmt.Println("No mi. check (actual) creds, project etc."); break }
    
    if verbose { fmt.Println("MI:"+mi.GetName()+" (Created: "+mi.GetCreationTimestamp()+")") }
    var cl int = mic.Classify(mi)
    if verbose { fmt.Println(verdict[cl]) }
    miclstat[cl]++
    if MIs.ToBeDeleted(cl) {
      todel++
      if verbose { fmt.Printf("DELETE %s\n", mi.GetName()) } // Also in DRYRUN
      mimi := MIMI{miname: mi.GetName(), class: cl}
      // Store MI to list
      //delarr = append(delarr, mi)
      delarr = append(delarr, mimi)
    } else {
      if verbose { fmt.Printf("KEEP %s\n", mi.GetName()) }
    }
    if verbose { fmt.Printf("============\n") }
    totcnt++
  }
  // Dry-run - terminate here
  if !mic.DelOK { fmt.Printf("# Dry-run mode, DelOK = %t (%d to delete)\n", mic.DelOK, todel); miclstat_out(mic, miclstat);return; }
  if len(delarr) == 0 { fmt.Printf("# Nothing to Delete (DelOK = %t, %d to delete)\n", mic.DelOK, todel); miclstat_out(mic, miclstat);return; }
  // Delete items from delarr - either serially or in chunks
  if mic.ChunkDelSize == -1 {
    mimilist_del_chan(mic, delarr)
  } else if mic.ChunkDelSize == 0 {
    mimilist_del_serial(mic, delarr)
  } else {
    mimilist_del_chunk(mic, delarr)
  }
}

func miclstat_out(mic MIs.CC, miclstat map[int]int) {
  // Need: .UTC(). ?
  fmt.Printf("MI Class (keep/delete reasoning) stats (%s, %s):\n", mic.Project, time.Now().Format("2006-01-02T15:04:05-0700")); // \n%+v\n", miclstat (raw dump)
  for key, value := range miclstat {
    fmt.Println("Class:", verdict[key], "(",key,") :", value)
  }
}

 // Serial Delete
func mimilist_del_serial(mic MIs.CC, delarr []MIMI) {
    for _, mimi := range delarr { // i
      fmt.Printf("%s (%s)\n", mimi.miname, verdict[mimi.class]) // mi.GetName()
      if mic.DelOK {
        //OLD:rc := mic_delete_mi(&mic, mi.GetName());
        rc := mic_delete_mi(&mic, &mimi);
        if rc != 0 { }
      }
    }
}
// Chunk / Parallel delete
func mimilist_del_chunk(mic MIs.CC, delarr []MIMI) { // ...
    //return
    sasize := mic.ChunkDelSize
    chunks := chunk_mimi(delarr, sasize )
    if chunks == nil { return }
    // Modeled along 
    for i, chunk := range chunks {
      fmt.Printf("Chunk %d (of %d items): %+v\n", i, sasize, chunk);
      var wg sync.WaitGroup
      for _, item := range chunk{
        wg.Add(1)
        // Workaround for go remembering the last value for the pointer / chunk (here)
        // CB closure gets the *current* value in iteration and forces the actual value to be passed
        // (mnot the last value of iteration).
        func (item MIMI) { go mic_delete_mi_wg(&mic, &item, &wg) } (item)
        // test sleeping to not hit API throttling.
        // https://golang.cafe/blog/golang-sleep-random-time.html
        rand.Seed(time.Now().UnixNano())
        //ms := time.Millisecond*100
        //ms = time.Millisecond*(50 + rand.Intn(100))
        ms := time.Duration(rand.Intn(100)+50) * time.Millisecond
        fmt.Printf("SLEEP: %d\n", ms);
        time.Sleep(ms)
      }
      wg.Wait()
      fmt.Printf("Waited for chunk to complete\n");
    }
    return
}
// Delete items directly using original lnear delarr using channels (underneath).
// This runs N items *all* the time instead waiting a "chunk" (where items could take different time
  // to complete individually, waiting for the longest processing one) to complete.
func mimilist_del_chan(mic MIs.CC, delarr []MIMI) {
  // TODO: Use NewWorkerPool() to have control over config
  wpool, _ := workerpool.NewWorkerPool(&workerpool.Config{WorkerLimit: mic.WorkerLimit, WorkerTimeoutSeconds: 0}) // NewDefaultWorkerPool()
  var wg sync.WaitGroup
  wg.Add(len(delarr))
  for _, mimi := range delarr {
    // Inner func gets assigned to work - so it *is* (inner) anon func
    go func(mimi MIMI) {
      defer wg.Done()
      wpool.RequestWork(func() {mic_delete_mi(&mic, &mimi)}) 
    }(mimi)
    
  }
  wg.Wait()
  fmt.Printf("Channel based processing completed !\n");
}
func mic_delete_mi(mic * MIs.CC, mimi * MIMI) int { // mi *computepb.MachineImage
  err := mic.Delete(mimi.miname) // mi.GetName()
  if err != nil {
   fmt.Printf("Error Deleting MI: %s\n", mimi.miname) //  mi.GetName()
   return 1
  } else {
   fmt.Printf("Deleted %s\n", mimi.miname) //  mi.GetName()
   return 0
  }
  //fmt.Printf("Should have deleted %s. Set DelOK (MI_DELETE_EXEC) to actually delete.\n", mi.GetName())
  return 0
}

func key_list() {
  ctx := context.Background()
  iamService, err := iam.NewService(ctx)
  if err != nil { fmt.Println("No Service"); return }
  var pname string = mic.Project
  acct := os.Getenv("GCP_SA")
  pname = os.Getenv("GCP_SA_PROJECT") // override
  if acct == "" { fmt.Println("No GCP_SA"); return }
  if pname == "" { fmt.Println("No GCP_SA_PROJECT"); return }
  sapath := fmt.Sprintf( "projects/%s/serviceAccounts/%s", pname,  acct)
  resp, err := iamService.Projects.ServiceAccounts.Keys.List(sapath).Context(ctx).Do()
  if err != nil { fmt.Printf("No Keys %v\n", err); return }
  //fmt.Println("Got:", resp) // iam.ListServiceAccountKeysResponse
  fmt.Printf("%T\n", resp) // import "reflect" fmt.Println(reflect.TypeOf(tst))
  for _, key := range resp.Keys {
    fmt.Printf("%T\n", key) // iam.ServiceAccountKey
    fmt.Printf("%v Exp.: %s\n", path.Base(key.Name), key.ValidBeforeTime)
  }
  // Also: SignJwtRequest, but: https://cloud.google.com/iam/docs/migrating-to-credentials-api
}
