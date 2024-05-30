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
// #NOT:go build grsc.go
// go build
// # Look for binary goclowdy
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
	"reflect"
	"regexp"
	"time"

	"google.golang.org/api/iterator"

	//compute "cloud.google.com/go/compute/apiv1" // Used only in lower levels
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	//goc "VMs"
	//macv "MIs"
	GPs "github.com/ohollmen/goclowdy/GPs"
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

	//"regexp"

	"github.com/ohollmen/goclowdy/workerpool"
	"golang.org/x/exp/slices" // 1.21 has this built-in
)

var verdict = [...]string{"KEEP to be safe", "KEEP NEW (< KeepMinH)", "KEEP (MID-TERM, WEEKLY)", "DELETE (MID-TERM)", "DELETE OLD (> KeepMaxH)", "KEEP-NON-STD-NAME", "KEEP (MID-TERM MONTHLY)"}
var envcfgkeys = [...]string{"GCP_PROJECT","GOOGLE_APPLICATION_CREDENTIALS","MI_DELETE_EXEC","MI_STDNAME", "MI_CHUNK_DEL_SIZE"}
var wdnames = []string{"SUN","MON","TUE","WED","THU","FRI","SAT"}
// Default MI client config
// 168 h = 1 = week, (24 * (365 + 7)) hours = 1 year,  weekday 5 = Friday (wdays: 0=Sun... 6=Sat). KeepMaxH (days) 365 => 548 (18 m)
var mic  MIs.CC = MIs.CC{Project: "",  WD_keep: 6, MD_keep: 1, KeepMinH: 168,  KeepMaxH: (24 * (548 + 7)), } // tnow: tnow, tloc: loc TZn: "Europe/London"
var bindpara MIs.CC = MIs.CC{}
// Machine image mini-info. Allow deletion to utilize this (for reporting output). Add creation time ?
type MIMI struct {
  miname string
  class int
}
// Subcommand callback (No params for now)
type SCCB = func() //error
type SubComm struct {
  cmd string
  name string
  cb SCCB
  //hide bool
  // TODO: Simple slice of paramname=paramtype mappings (how to bind struct mem - by reflection ?) ?
  // OR Introspect
  // For convention see: https://perldoc.perl.org/Getopt::Long (s=string,i=int,f=float,""=bool=>no value)
  // opts []string
}
var scomms = []SubComm{
  {"vm_mi_list", "VM list", vm_ls}, // Machine image
  {"midel",      "Delete Machine images", mi_del},
  {"keylist",  "List SA Keys from a GCP Project", key_list},
  {"env",      "List goclowdy (utility) config and environment", env_ls},
  {"subarr",   "Subarray Test", subarr_test},
  {"milist",   "List Machine Images (With time stats)", mi_list},
  {"vmbackup", "Backup VMs from a project", vm_backup},
  {"projlist", "List projects", proj_list},
  {"projsvmbackup", "List Projects and VMs", projsvmbackup},
  //{"","",},//{"","",},

}
// Env-to-member mapping ( "GOOGLE_APPLICATION_CREDENTIALS" : "CredF")
var argmap = map[string]string{"GCP_PROJECT": "Project", "GCP_BULL": "None"}
var clpara = map[string]string{"project": "", "appcreds": ""}
// 
func usage(msg string) {
  //subcmds := "vm_mi_list,midel,keylist,env"
  if msg != "" { fmt.Printf("Usage: %s\n", msg); }
  fmt.Println("Pass one of subcommands: "); // +subcmds
  for _, sc := range scomms {
    fmt.Printf("- %s - %s\n", sc.cmd, sc.name)
  }
}
// Extract subcommand (op) and remove it from os.Argv (for flags module)
func args_subcmd() string {
  if len(os.Args) < 2 { return "" } // No room for a subcommand
  if mic.Debug { fmt.Printf("Args: %v\n", os.Args) }
  op := os.Args[1:2][0]
  if mic.Debug { fmt.Printf("OP: %v\n", op); }
  os.Args = slices.Delete(os.Args, 1, 2)
  if mic.Debug { fmt.Printf("Args: %v\n", os.Args) }
  return op;
}

var Filter = ""

// OLD-TODO: Loop trough arg-keys, populate map w. ""-values.
// TODO: Possibly do tiny bit of reflection here to detect type ?
func args_bind() { // clpara map[string]string
  // OLD: Use separate (initally empty) bindpara MIs.CC (copy filled-out members by mem-by-mem copy after).
  // NEW: Use mic and time parsing (flag.Parse()) after env-sourcing in Init.
  // TODO: Establish cases (or if-else) here for sub-command specific type-binding
  ////// Most of  MI based commands /////////////
  flag.StringVar(&mic.Project, "project", "", "GCP Cloud project (string) id")
  flag.StringVar(&mic.CredF,   "appcreds", "", "GCP Cloud Application Credentials (string)")
  flag.BoolVar(&mic.Debug,     "debug", false, "Set debug mode.")
  flag.StringVar(&Filter,      "filter", "", "Filter for Project Operations")
  //flag.IntVar(p *int, name string, value int, usage string)

  // This does not work based on Go dangling pointer-policies
  //flag.StringVar(&clpara["project"], "project", "", "GCP Cloud project (string) id")
  //flag.StringVar(&clpara["appcreds"], "appcreds", "", "GCP Cloud Application Credentials (string)")
}
// OLD: Override CLI originated params last (after cfg, env)
func args_override() { // UNUSED
  fmt.Printf("Project=%s\n", bindpara.Project)
  if bindpara.Project != "" { mic.Project = bindpara.Project; } // After config_load() ?
}
// Override env based on map and (member name) reflect. How to pass generic pointer (any ?) ?
// Interface (is-a-pointer) ? Also any is an interface.
func args_env_merge(e2sm map[string]string, mystruct any ) int { // UNUSED (replaced by env.Set())
  cnt := 0
  // Validate that mystruct IS-A struct ! reflect.Valueof() vs. refelct.TypeOf() ??
  if reflect.TypeOf(mystruct).Kind() != reflect.Struct { fmt.Printf("Not a struct !\n"); return -1; }
  //  fmt.Println(reflect.ValueOf(e).Field(i))
  //}
  s := reflect.ValueOf(&mystruct).Elem() // w/o & - get: panic: reflect: call of reflect.Value.Elem on struct Value
  //s := reflect.Indirect(reflect.ValueOf(&mystruct))
  for k, v := range e2sm {
    //confMap[v.Key] = v.Value
    ev := os.Getenv(k);
    if (ev == "") { fmt.Printf("No value for env: '%s'\n", k); continue; }
    fmt.Printf("kv:%s/%s\n", k, v)
    //fmt.Println("val(0)", reflect.ValueOf(mystruct).Field(0))
    f := s.Elem().FieldByName(v) // .Interface() // OLD: fval (works till f.SetString())
    //f := s.FieldByName(v) // N/A either: panic: reflect: call of reflect.Value.FieldByName on interface Value
    // Weird, but correct way
    if f == (reflect.Value{}) { fmt.Printf("Could not reflect/lookup member: '%s'\n", v); continue; }
    //if f.Kind() != "string" { fmt.Printf("Only string fields supported at this time (Not: '%s')", f.Kind()); continue; }
    fmt.Printf("Original val('%s', type: '%s'): %s\n", v, f.Kind(), f.Interface())
    // TODO: Based on type (f.Kind() ?)
    // panic: reflect: reflect.Value.SetString using unaddressable value
    // https://stackoverflow.com/questions/48568542/update-an-attribute-in-a-struct-with-reflection
    //f.SetString(ev) // 

    cnt++
  }
  return cnt; // # changed/overriden
}
func main() {
  //ctx := context.Background()
  
  if len(os.Args) < 2 { usage("Subcommand missing !"); return; } // fmt.Println("Pass one of subcommands: "+subcmds); return
  op := args_subcmd()
  args_bind() // OLD: clpara. Bind here, call flag.Parse() later.
  
  //flag.Parse() // os.Args[2:] From Args[2] onwards
  //fmt.Printf("CL-ARGS(map): %v\n", clpara);
  
  
  //if () {}
  //pname := os.Getenv("GCP_PROJECT")
  //if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  //if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  config_load("", &mic);
  //NOT:args_override() // OLD: Worked on bindpara. Would need to call after mic.Init()
  idx := slices.IndexFunc(scomms, func(sc SubComm) bool { return sc.cmd == op })
  scomms[idx].cb()
  return
}

func env_ls() {
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
    if mic.NameRE != nil { fmt.Printf("# MI-RE:\n# - As Str:  %s\n# - From RE: %s\n", mic.NameREStr, mic.NameRE); }
}

// Backup VMs from a GCP Project
func vm_backup() {
  // Need vmc and mic
  // vmc ...
  vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF};
  err := vmc.Init() // Will pickup 
  if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
  
  // Same workaround as for mi_list
  flag.Parse()
  vmc.Project = mic.Project;
  fmt.Printf("vmc Project: %s\n", vmc.Project);
  // mic ...
  rc := mic.Init()
  if rc != 0 { fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  fmt.Printf("mic Project: %s\n", mic.Project);
  
  all := vmc.GetAll() //; fmt.Println(all)
  // Filter the superset "all" (e.g. "apache.*"). Note: Improve/extend initial slim / narrow VM name based fitering implementation
  namepatt := os.Getenv("GCP_VM_NAMEPATT") // TODO: param from ...
  if namepatt != "" {
    fmt.Printf("Got namepatt (GCP_VM_NAMEPATT): %s\n", namepatt)
    NameRE, err := regexp.Compile(namepatt)
    if err != nil { fmt.Printf("namepatt (RE) not comiled: %s\n", err); return; }
    var ftd []*computepb.Instance
    for _, vm := range all { // FindStringSubmatch
      if NameRE.FindString(vm.GetName()) != "" { ftd = append(ftd, vm); }
    }
    all = ftd
  }
  icnt := len(all)
  if icnt == 0 { fmt.Println("No VMs found (after filtering)"); return }
  
  fmt.Printf("%d VMs to backup\n", icnt);
  mic.Debug = true
  //fmt.Printf("# Got %v  Instances\n", icnt) // Initial ... (Filtering ...)
  //return;
  // Iterate VM:s
  var wg sync.WaitGroup
  mic.Wg = &wg // Set as shared (ptr)
  //namesuffbase := mic.DatePrefix("testsuffix", nil) // mic.Tnow().Format("2006-01-02")+"-testsuffix" // Appended to name, should be e.g. ISO date + "-" + daily/weekly/monthly
  //namesuffbase = "testsuffix"
  //fmt.Println("Use name suffix "+namesuffbase); return; 
  // Initially: all listed in VM-to-backup: ..., but only 3 or 4 show "MI name to use: ..." see: 
  // 
  cb := func(vm *computepb.Instance) {
    //defer wg.Done()
    // go - NOT needed here if stated before
    totake := mic.MIsToTake(nil) // 1..3
    if totake > 0 { fmt.Printf("Take (bitwise): %d\n", totake); }
    sarr   := MIs.BitsToTimesuffix(totake)
    wg.Add(len(sarr)) // if wg
    for _, suffitem := range sarr {
      namesuffbase := mic.DatePrefix(suffitem, nil)
      go mic.CreateFrom(vm, namesuffbase)
    }
  }
  // Note: Match wg.Add(1) / wg.Done()
  for _, vm := range all{ // Instance
    
    fmt.Println("VM-to-backup: ", vm.GetName()) // if mc.Debug
    
    //go cb(item, icfg.Userdata);
    // 2nd: altsuff ... will be appended staring w. '-'
    //go
    cb(vm) // mic.CreateFrom(vm, "testsuffix")
    //wg.Done() // if wg. NOTE here, but at the end of goroutine task completion

  }
  fmt.Println("Done w. launching. Starting to wait ...");
  wg.Wait()
  mic.Wg = nil // Set null for client reuse (ONLY after Wait)
}
// List VMs. Set GCP_PROJECT and GOOGLE_APPLICATION_CREDENTIALS as needed (or get from config)
func vm_ls() { // pname string
    //ctx := context.Background()
    //mic.Init() // OLD: Init due to side effects affecting vmc
    // test overlapping sysm (old: vs). Borrow params from mic.
    vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF} //  
    err := vmc.Init()
    if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
    all := vmc.GetAll() //; fmt.Println(all)
    icnt := len(all)
    if icnt == 0 { fmt.Println("No VMs found"); return }
    fmt.Printf("# Got %v  Instances\n", icnt) // Initial ... (Filtering ...)
    //stats := VMs.CreateStatMap(all)
    //fmt.Printf("%v", stats)
    // Test for daily MI. This is now embedded to mic.CreateFrom() logic.
    //mic.Init() // If bottom MI lookup enabled
    for _, it := range all{ // Instance
      fmt.Println(""+it.GetName())
      continue;
      // OLD: Check presense of standard-name (ISO-time-appended to hostname) MI Image
      /*
      in := MIs.StdName(it.GetName())
      fmt.Println("STD Name:", in)
      mi := mic.GetOne(in)
      if mi != nil  {
        fmt.Println("Found image: ", mi.GetName())
      } else { fmt.Println("No (std) image found for : ", it.GetName()) }
      */
    }
    return
}
// New MI List with statistical count of MIs per time eras (now...1w, 1w...1y).
// Mix of access to VMs (find all) and MIs (See: vm_ls() for "ingredients" of solution)
// The mic.HostREStr must be a pattern that captures the hostname part from the machine image as capture group 1
// e.g. export MI_HOSTPATT='^(\w+-\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3})'
func mi_list() {
  //ctx := context.Background()
  //////// VMs //////////
  fmt.Printf("Proj: %s\n", mic.Project);
  vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF} //  
  err := vmc.Init()
  // Note: the flag.Parse() and Project reassign are workaround for inherent state problems for the
  // config -> env env.Set(cfg)-> CL flag.Parse() override seq.
  flag.Parse()
  vmc.Project = mic.Project;
  if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
  fmt.Printf("Proj: %s\n", vmc.Project);
  allvms := vmc.GetAll()
  if len(allvms) < 1 { fmt.Println("No VMs found"); return; }
  stats := VMs.CreateStatMap(allvms)
  if vmc.Debug { fmt.Printf("%v %v", allvms, stats) }
  //////// MIs ///////////
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  all := mic.GetAll()
  // Because we collect stats by hostname, the pattern to match must be there !
  if mic.HostREStr == "" { fmt.Printf("Warning: No HostREStr in environment (MI_HOSTPATT) or config !"); return; }
  if mic.HostRE == nil   { fmt.Printf("Warning: No HostRE pattern matcher (RE syntax error ?) !"); return; }
  totcnt := 0 // TODO: More diverse stats
  secnt := mic.HostRE.NumSubexp()
  fmt.Printf("Subexpressions: %d\n", secnt);
  for _, mi := range all {
    totcnt++
    t, err := mic.CtimeUTC(mi)
    if err != nil { fmt.Printf("MI C-TS not parsed !"); continue; }
    agehrs := mic.AgeHours2(t)
    if agehrs > float64(mic.KeepMaxH) { fmt.Printf("Too old\n"); continue; }
    if (mic.HostRE != nil) {
      m := mic.HostRE.FindStringSubmatch( mi.GetName() )
      if len(m) < 1 { fmt.Printf("No capture items for hostname matching (%s)\n", mi.GetName() ); continue; }
      fmt.Printf("HOSTMatch: %v, MINAME: %s AGE: %f\n", m[1], mi.GetName(), agehrs );
      mis, ok := stats[m[1]] // MI stat. Is this copy or ref to original ? https://golang.cafe/blog/how-to-check-if-a-map-contains-a-key-in-go
      //if mis.Mincnt > 100 {} // Dummy
      if !ok { fmt.Printf("No stats entry for captured VM name '%s'\n", m[1]); continue; }
      
      if agehrs <= float64(mic.KeepMinH)  { mis.Mincnt++; //stats[m[1]].Mincnt++
      } else if agehrs <= float64(mic.KeepMaxH) { mis.Maxcnt++ ; //stats[m[1]].Maxcnt++
      } // else { mis.Mincnt++;  mis.Maxcnt++ } // Last: test/debug
    } // NumSubexp
  }
  // The downside of having map value-struct as *pointer* is we see only mem-address in (raw Printf("%v", ...)) dump (!!!)
  //fmt.Printf("STATS: %v\n", stats)
  // Report Backup statistics per host (in current project). TODO: Populate Struct for JSON
  //var reparr []*VMs.MIStat // Empty/0-items, no pre-alloc
  reparr := make([]*VMs.MIStat, len(stats)) // Prealloc to right size (Use indexes)
  i := 0
  for _, stat := range stats {
    fmt.Printf("%s %d %d\n", stat.Hostname, stat.Mincnt, stat.Maxcnt);
    reparr[i] = stat // OLD: append(reparr, stat) // append() will append to current pre-alloc'd len of slice !!
    i++
  }
  jba, err := json.MarshalIndent(reparr, "", "  ") // ([]byte, error)

  fmt.Printf("%s\n", jba) // Ok w. []byte
  fmt.Printf("# %d Images from %s\n", totcnt, mic.Project);
}

// Delete machine images per given config policy.
// TODO: Possibly Convert to use getAll, except we want MIMI (not full computepb.MachineImage ents)
func mi_del() { // pname string
  ctx := context.Background()
  //config_load("", &mic); // Already on top
  
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic);  }
  fmt.Printf("Config (after init): %+v\n", &mic);
  if rc != 0 { fmt.Printf("Machine image module init failed: %d\n", rc); return }
  // Classification stats. Note: no wrapping make() needed w. element initialization
  miclstat := map[int]int{0: 0, 1:0, 2:0, 3:0, 4:0, 5:0}
  //TEST: miclstat_out(mic, miclstat); return;
  var maxr uint32 = 500 // 20
  if mic.Project == "" { fmt.Println("No Project passed"); return }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  //fmt.Println("Search MI from: "+cfg.Project+", parse by: "+time.RFC3339)
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No mi:s from "+mic.Project); return; }
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
    /////// Actual processing ////////
    // NEW: Use discovered UTC C-TS
    t, err := mic.CtimeUTC(mi)
    if err != nil { fmt.Println("Create-time not parsed !");break; }
    // OLD: mi.GetCreationTimestamp()
    if verbose { fmt.Println("MI:"+mi.GetName()+" (Created: "+t.Format(time.RFC3339)+" "+wdnames[t.Weekday()]+")") } //  
    var cl int = mic.Classify(mi)
    if verbose { fmt.Println(verdict[cl]) }
    miclstat[cl]++
    if MIs.ToBeDeleted(cl) {
      todel++
      if verbose { fmt.Printf("DELETE %s\n", mi.GetName()) } // Also in DRYRUN
      mimi := MIMI{miname: mi.GetName(), class: cl}
      // Store MI to list
      //delarr = append(delarr, mi) // OLD full mi
      delarr = append(delarr, mimi)
    } else {
      if verbose { fmt.Printf("KEEP %s\n", mi.GetName()) }
    }
    if verbose { fmt.Printf("============\n") }
    totcnt++
  }
  // Dry-run (or no ents) - terminate here
  if !mic.DelOK { fmt.Printf("# Dry-run mode, DelOK = %t (%d to delete)\n", mic.DelOK, todel); miclstat_out(mic, miclstat);return; }
  if len(delarr) == 0 { fmt.Printf("# Nothing to Delete (DelOK = %t, %d to delete)\n", mic.DelOK, todel); miclstat_out(mic, miclstat);return; }
  // Delete items from delarr - either serially or in chunks or using channels
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
  //return 0
}

func key_list() {
  ctx := context.Background()
  iamService, err := iam.NewService(ctx)
  if err != nil { fmt.Println("No Service: ", err); return }
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

func proj_list()  { // error ?
  flag.Parse()
  //fmt.Printf("Got Filter: %s\n", Filter);
  //var qmap = map[string]string{} // {"include": "true"}
  qmap := GPs.KvParse(Filter)
  qstr := GPs.Map2Query(qmap)
  if qstr != "" { fmt.Printf("Query: %s\n", qstr); }
  Projects := GPs.ProjectsList(qstr)
  fmt.Printf("# %d Projects retrieved by filter: '%s'\n", len(Projects), qstr)
  // Filter ?
  for _, project := range Projects {
    // fmt.Printf("%s =>\n%v\n", project.ProjectId, *project)
    fmt.Printf("%s\n", project.ProjectId)
  }
  return
}
// E.g. 352 => 60 (5s)
func projsvmbackup() {
  flag.Parse()
  qmap := GPs.KvParse(Filter)
  qstr := GPs.Map2Query(qmap)
  //qstr := "" // "labels.include=true"
  var vmc VMs.CC = VMs.CC{Project: ""} // CredF: ""
  err := vmc.Init();
  if err != nil { fmt.Printf("No vmc !"); return; }
  Projects := GPs.ProjectsList(qstr)
  if Projects == nil { fmt.Printf("No Projects Listed"); return; }
  pvms := GPs.ProjectsVMs(Projects, vmc)
  for _, pvm := range pvms {
    fmt.Printf("  - VM: %s/%s\n", pvm.Project.ProjectId, pvm.Vm.GetName());
  }
}
