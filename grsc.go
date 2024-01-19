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

	"github.com/ohollmen/goclowdy/workerpool"
)

var verdict = [...]string{"KEEP to be safe", "KEEP NEW (< KeepMinH)", "KEEP (MID-TERM, WEEKLY)", "DELETE (MID-TERM)", "DELETE OLD (> KeepMaxH)", "KEEP-NON-STD-NAME"}
var envcfgkeys = [...]string{"GCP_PROJECT","GOOGLE_APPLICATION_CREDENTIALS","MI_DELETE_EXEC","MI_STDNAME", "MI_CHUNK_DEL_SIZE"}
// Default MI client config
var mic  MIs.CC = MIs.CC{Project: "",  WD_keep: 5, KeepMinH: 168,  KeepMaxH: (24 * (365 + 7)), TZn: "Europe/London"} // tnow: tnow, tloc: loc
// Machine image mini-info. Allow deletion to utilize this (for reporting output). Add creation time ?
type MIMI struct {
  miname string
  class int
}
func main() {
  //ctx := context.Background()
  subcmds := "vm_mi_list,midel,keylist,env"
  if len(os.Args) < 2 { fmt.Println("Pass one of subcommands: "+subcmds); return }
  //if () {}
  pname := os.Getenv("GCP_PROJECT")
  if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  if os.Args[1] == "vm_mi_list" {
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
    config_load("", &mic);
    mic.Init();
    jb, _ := json.MarshalIndent(&mic, "", "  ")
    fmt.Printf("# Config as JSON (After config load and Init()):\n%s\n", jb)
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
  } else { fmt.Println("Pass one of subcommands: "+subcmds); return }
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

// Delete machine images per given config policy
func mi_del(pname string) {
  ctx := context.Background()
  // midel := os.Getenv("MI_DELETE_EXEC")
  // https://pkg.go.dev/regexp
  //var stdnamere * regexp.Regexp // Regexp
  //var err error
  // E.g. "^\\w+-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}$" (in Go runtime)
  // E.g. "^[a-z0-9-]+?-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}$" // No need for 1) \\ before [..-] 2) \ before [ / ]
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
  
  config_load("", &mic);
  
  rc := mic.Init()
  fmt.Printf("Config (after init): %+v\n", &mic);
  // TODO: Configdump
  if false {
    
  }
  //return;
  if rc != 0 { fmt.Printf("Machine image module init failed: %d\n", rc); return }
  // EARLIER: if midel != "" { mic.DelOK = true; } // Non-empty => DELETE
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
  verbose := true
  for {
    //fmt.Println("Next ...");
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break }
    if mi == nil {  fmt.Println("No mi. check (actual) creds, project etc."); break }
    
    if verbose { fmt.Println("MI:"+mi.GetName()+" (Created: "+mi.GetCreationTimestamp()+")") }
    var cl int = mic.Classify(mi)
    if verbose { fmt.Println(verdict[cl]) }
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
  if !mic.DelOK { fmt.Printf("# Dry-run mode, DelOK = %t (%d to delete)\n", mic.DelOK, todel); return; }
  // Delete items from delarr - either serially or in chunks
  if mic.ChunkDelSize == -1 {
    mimilist_del_chan(mic, delarr)
  } else if mic.ChunkDelSize == 0 {
    mimilist_del_serial(mic, delarr)
  } else {
    mimilist_del_chunk(mic, delarr)
    
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
        time.Sleep(time.Millisecond*100)
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
  if err != nil { fmt.Printf("No Keys %v\n", err); return }
  //fmt.Println("Got:", resp) // iam.ListServiceAccountKeysResponse
  fmt.Printf("%T\n", resp) // import "reflect" fmt.Println(reflect.TypeOf(tst))
  for _, key := range resp.Keys {
    fmt.Printf("%T\n", key) // iam.ServiceAccountKey
    fmt.Printf("%v Exp.: %s\n", path.Base(key.Name), key.ValidBeforeTime)
  }
  // Also: SignJwtRequest, but: https://cloud.google.com/iam/docs/migrating-to-credentials-api
}
