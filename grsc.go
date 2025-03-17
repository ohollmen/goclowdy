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
	//"context"
	"fmt"
	"os" // Args
	"reflect"
	"regexp"
	//"time"

	//"google.golang.org/api/iterator"

	//compute "cloud.google.com/go/compute/apiv1" // Used only in lower levels
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	//goc "VMs"
	//macv "MIs"
	GPs "github.com/ohollmen/goclowdy/GPs"
	MIs "github.com/ohollmen/goclowdy/MIs"
	VMs "github.com/ohollmen/goclowdy/VMs"
  //"github.com/ohollmen/goclowdy/workerpool"
	OrgTree "github.com/ohollmen/goclowdy/OrgTree"

	//"github.com/ohollmen/goclowdy"
	//"goclowdy/vm/VMs"
	//"goclowdy/mi/MIs"
	//MIs "goclowdy/mi"
	//VMs "goclowdy/vm"
	//"goclowdy/VMs"
	//"goclowdy/MIs"
	// Base

	// NEW
	//"regexp" // responsibility moved to MIs
	"encoding/json"
	"sync" // go get -u golang.org/x/sync

	//"math/rand"

	"flag"

	//"regexp"

	
	"golang.org/x/exp/slices" // 1.21 has this built-in
  // "cloud.google.com/go/storage" // GCS, See: https://cloud.google.com/storage/docs/samples/storage-upload-file#storage_upload_file-go
  //"path/filepath" // Win + UNIX
  "path" // "/" based paths (std. lib)
  // Required by WalkDir
  //"io/fs" // for DirEntry present in WalkDir cb signatures
  //"path/filepath" // Walk / WalkDir and  abs-to-rel by Rel()
  //"bytes" // Fors splitting bytearrays bytes.Split()
  //"path/filepath" //
)

var verdict = [...]string{"KEEP to be safe", "KEEP NEW/RECENT (< KeepMinH)", "KEEP (MID-TERM, WEEKLY)", "DELETE (MID-TERM)", "DELETE OLD (> KeepMaxH)", "KEEP-NON-STD-NAME", "KEEP (MID-TERM, MONTHLY)"}
var envcfgkeys = [...]string{"GCP_PROJECT","GOOGLE_APPLICATION_CREDENTIALS","MI_DELETE_EXEC","MI_STDNAME", "MI_CHUNK_DEL_SIZE"}
var wdnames = []string{"SUN","MON","TUE","WED","THU","FRI","SAT"} // used by mid_del
// Default MI client config (provide sane default and example, all time units are hours).
// - KeepMinH 168 h = 1 week
// - Weekly, Monthly: WD_keep - e.g. weekday 6 = Saturday (wdays: 0=Sun... 6=Sat), MD_keep - e.g. 1 = First (always exists)
// - KeepMaxH examples: (24 * (365 + 7)) hours = 1 year (365 d), (24 * (548 + 7)) h => 1.5 years (548 d => 18 mon, 13320 h) 
var mic  MIs.CC = MIs.CC{Project: "",  WD_keep: 6, MD_keep: 1, KeepMinH: 168,  KeepMaxH: (24 * (548 + 7)), } // tnow: tnow, tloc: loc TZn: "Europe/London"
//var bindpara MIs.CC = MIs.CC{}

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
  // optkeys []string // Opts the SubComm is interested in (mandatory / optional ?)
  // opts map[string]:string // Populate a map of package (CLI para) globals ??
}
// Handlers for subcommands. These are called by top-level main entrypoint and should comply to
// same signature (refactor handles hand-in-hand)
var scomms = []SubComm{
  {"vmlist",    "VM list", vm_ls}, // OLD: Machine image / vm_mi_list
  {"midel",     "Delete Machine images (per retention config, use --project for proj. override, --delok to actually delete)", mi_del},
  {"midelmax",     "Delete Machine images beyond max age (only)", mi_del_max},
  {"keylist",   "List SA Keys from a GCP Project", key_list},
  {"keycheck",   "Check SA Key given in Env. GOOGLE_APPLICATION_CREDENTIALS", key_check},
  
  {"key_gen",   "BETA: Renew / Generate new SA Key based on old key", key_gen},
  {"env",       "List goclowdy (utility) config and environment", env_ls},
  {"subarr",    "Subarray Test", subarr_test},
  {"mitstats",  "List Machine Images (With time stats)", mi_time_stats},
  {"milist",    "List Machine Images", mi_list2},
  {"vmbackup",  "Backup VMs from a single project (Use overriding --project, --filter (by VM name) and --suffix (additional suffix) as needed)", vm_backup},
  {"projlist",  "List projects (in org, Use --ansible to output as ansible inventory, use --filter to do label filtering)", proj_list}, // labelfilter
  {"projsvmbackup", "Backup VMs from Org (Use --filter to perform labels based filtering, --suffix at add custom suffix)", projsvmbackup}, // labelfilter
  {"projsvmlist", "List VMs from Org along their project (Use --filter to perform labels based filtering)", projsvmbackup}, // labelfilter
  {"orgtree",   "Dump OrgTree as JSON", orgtree_dump},
  {"projvmstop", "Stop VMs in a Project (Use --project and optional --filter)", proj_shutdown},
  //{"grep","", file_filter},
  //{"","",},
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
    if sc.name == "" { continue; }
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
// CL Params as main-package-global scalars (for now keep these as scalars as maintaining these in particular packages has shown
// to be difficult)
// var Project = ""
// Generic name filter for various operations (e.g.: name patter of VM:s to back up)
var Filter = ""
var Labelfilter = ""
var Suffix = ""
var Prefix = "" // for e.g. projlist
var Ansible = false;
var Delok = false;
var FilterRE regexp.Regexp;
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
  flag.StringVar(&Filter,      "filter", "", "Label Filter OR VM name filter for Project/VM-MI Operations")
  flag.StringVar(&Labelfilter, "labelfilter", "", "Project Label Filter for Org/Project/VM-MI Operations")
  flag.StringVar(&Suffix,      "suffix", "", "Additional Suffix for Machine image (after vmname + IS date)")
  flag.StringVar(&Prefix,      "prefix", "", "Prefix (string) to use for projects listing (e.g. --prefix '    - ' for YAML list)")
  flag.BoolVar(&Ansible,      "ansible", false, "Create projectlist as ansible inventory.")
  flag.BoolVar(&Delok,        "delok", false, "Indicate that deletion is okay (e.g. MI).")
  //flag.IntVar(p *int, name string, value int, usage string)

  // This does not work based on Go dangling pointer-policies
  //flag.StringVar(&clpara["project"], "project", "", "GCP Cloud project (string) id")
  //flag.StringVar(&clpara["appcreds"], "appcreds", "", "GCP Cloud Application Credentials (string)")
}
// OLD: Override CLI originated params last (after cfg, env)
//func args_override() { // UNUSED
//  fmt.Printf("Project=%s\n", bindpara.Project)
//  if bindpara.Project != "" { mic.Project = bindpara.Project; } // After config_load() ?
//}

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
var gop = ""
func main() {
  //ctx := context.Background()
  
  if len(os.Args) < 2 { usage("Subcommand missing !"); return; } // fmt.Println("Pass one of subcommands: "+subcmds); return
  op := args_subcmd()
  gop = op // Assign to globally usable counterpart
  args_bind() // OLD: clpara. Bind here, call flag.Parse() later (In handlers, esp after config_load()).
  
  //flag.Parse() // os.Args[2:] From Args[2] onwards
  //fmt.Printf("CL-ARGS(map): %v\n", clpara);
  
  
  //if () {}
  //pname := os.Getenv("GCP_PROJECT")
  //if pname == "" { fmt.Println("No project indicated (by GCP_PROJECT)"); return }
  //if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" { fmt.Println("No creds given by (by GOOGLE_APPLICATION_CREDENTIALS)"); return }
  config_load("", &mic); // TODO: Pass all possible ents that may be populated ?
  //NOT:args_override() // OLD: Worked on bindpara. Would need to call after mic.Init()
  idx := slices.IndexFunc(scomms, func(sc SubComm) bool { return sc.cmd == op })
  if (idx > len(scomms)) || (idx < 0) { fmt.Printf("%s - No such subcommand (idx=%d)\n", op, idx); return; }
  // Create item to pass the handler (copied + extended variant of discovered SubComm scomms[idx] ???)
  // opnode := scomms[idx] /// ... and add props ?
  // Run current/discovered subcommand. TODO: pass
  scomms[idx].cb()
  return
}

// Filter VMSet (passed as all) by name pattern string.
// Return filtered array (if RE fails to compile, return empty set)
// TODO: Similar filtering by vm.Labels (map[string]string) ... if vm.Labels[k] == v {...} (match all (AND) in criteria map)
func vmset_filter(all []*computepb.Instance, namepatt string) []*computepb.Instance {
  if namepatt == "" { return all; }
  var ftd []*computepb.Instance // New name-filtered array
  fmt.Printf("Got namepatt (form GCP_VM_NAMEPATT or --filter): '%s'\n", namepatt)
  NameRE, err := regexp.Compile(namepatt)

  if err != nil { fmt.Printf("Error: namepatt (RE) not compiled: %s\n", err); return ftd; }
    
  for _, item := range all { // FindStringSubmatch
    if NameRE.FindString(item.GetName()) != "" { ftd = append(ftd, item); }
  }
  return ftd;
}

func miset_filter(all []*computepb.MachineImage, namepatt string) []*computepb.MachineImage {
  if namepatt == "" { return all; }
  var ftd []*computepb.MachineImage // New name-filtered array
  fmt.Printf("Got namepatt (form GCP_VM_NAMEPATT or --filter): '%s'\n", namepatt)
  NameRE, err := regexp.Compile(namepatt)

  if err != nil { fmt.Printf("Error: namepatt (RE) not compiled: %s\n", err); return ftd; }
    
  for _, item := range all { // FindStringSubmatch
    if NameRE.FindString(item.GetName()) != "" { ftd = append(ftd, item); }
  }
  return ftd;
}
// Show config (conf-file, env, CLI params merged)
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

// Backup VMs from a (single) GCP Project.
// Note here --filter does VM name based filtering (NOT labels), --suffix works "normally"
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
  // Consider the --suffix passed from the command line
  if Suffix != "" { fmt.Printf("Overriding suffix for the MI backup(ws): %s\n", Suffix); }
  // mic ...
  rc := mic.Init()
  if rc != 0 { fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  fmt.Printf("mic Project: %s\n", mic.Project);
  // TODO: Consider --filter from CLI (e.g. filter after getting all)
  all := vmc.GetAll() //; fmt.Println(all)
  initcnt := len(all)
  if initcnt < 1 { fmt.Printf("No VM:s gotten from project (%s)\n", vmc.Project); return }
  // Filter down the superset "all" (e.g. by "apache.*"). Note: Improve/extend initial slim / narrow VM name based fitering implementation
  namepatt := os.Getenv("GCP_VM_NAMEPATT") // TODO: param from ...
  if Filter != "" { namepatt = Filter; }
  all = vmset_filter(all, namepatt);
  icnt := len(all)
  if icnt == 0 { fmt.Println("No VMs found (after filtering)"); return }
  
  fmt.Printf("%d VMs to backup\n", icnt);
  mic.Debug = true
  //fmt.Printf("# Got %v  Instances\n", icnt) // Initial ... (Filtering ...)
  //return;
  /////////////////// Backup VM:s ///////////////////
  
  // For simple scenario remains constant across all items
  namesuffbase := mic.DatePrefix(Suffix, nil) // mic.Tnow().Format("2006-01-02")+"-testsuffix" // Appended to name, should be e.g. ISO date + "-" + daily/weekly/monthly
  //namesuffbase = "testsuffix"
  fmt.Println("Use name suffix (refined): "+namesuffbase); // return; 
  // Multi-backup: Initially: all listed in VM-to-backup: ..., but only 3 or 4 show "MI name to use: ..." see: 
  totake := mic.MIsToTake(nil) // 1..3
  if totake > 0 { fmt.Printf("Take (bitwise): %d\n", totake); }
  sarr   := MIs.BitsToTimesuffix(totake) // suffix array
  // N name-variants for each vm (based on sarr/suffix array => suffitem). Do not use constant-across-all-vms suffix (namesuffbase) here.
  cb_multi := func(vm *computepb.Instance, wg * sync.WaitGroup, dummy string) {
    //NOT: defer wg.Done() // Done in CreateFrom()
    // go - NOT needed here if stated before
    wg.Add(len(sarr)) // if wg
    for _, suffitem := range sarr {
      namesuffbase_t := mic.DatePrefix(suffitem, nil)
      go mic.CreateFrom(vm, namesuffbase_t, "")
    }
  }
  // Do a simple / single backup with suffix passed from CLI
  cb_simple := func(vm *computepb.Instance, wg * sync.WaitGroup, altsuff string) {
    wg.Add(1)
    go mic.CreateFrom(vm, altsuff, "") // OLD: namesuffbase
  }
  // Note: Match wg.Add(1) / wg.Done()
  cb := cb_multi; // Default
  if Suffix != "" { cb = cb_simple; }
  if (cb == nil) { return; }
  /////// Multi-VM backup client ///////////////
  mic.CreateFromMany(all, cb, namesuffbase)
}
// List VMs from a single project. Set GCP_PROJECT and GOOGLE_APPLICATION_CREDENTIALS as needed (or get from config)
func vm_ls() { // pname string
    flag.Parse() // parse for --filter
    //ctx := context.Background()
    //mic.Init() // OLD: Init due to side effects affecting vmc
    // test overlapping sysm (old: vs). Borrow params from mic.
    vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF} //  
    err := vmc.Init()
    if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
    all := vmc.GetAll() //; fmt.Println(all)
    icnt := len(all)
    if icnt == 0 { fmt.Println("No VMs found"); return }
    fmt.Printf("# Got %v  Instances (from project '%s')\n", icnt, vmc.Project) // Initial ... (Filtering ..)
    all = vmset_filter(all, Filter); //  Allow filter
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

func mi_list2() {
  //fmt.Printf("Proj: %s\n", mic.Project);
  flag.Parse()
  //vmc.Project = mic.Project;
  //if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
  //fmt.Printf("Proj: %s\n", vmc.Project);
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  all := mic.GetAll()
  all = miset_filter(all, Filter); // Name (RegExp) filter
  fmt.Println("[");
  for _, mi := range all {
    //fmt.Println(mi.GetName());
    // Get ISO date
    t, _ := mic.CtimeUTC(mi);
    cls := mic.Classify(mi);
    fmt.Printf(" {\"title\": \"%s\", \"from\": \"%s\", \"to\": \"%s\", \"cls\": %d},\n", mi.GetName(), t.Format("2006-01-02"), t.Format("2006-01-02"), cls);
  }
  fmt.Println("{}]");
}
// New MI List with statistical count of MIs per time eras (now...1w, 1w...1.Xy, > 1.Xy).
//  Report Backup statistics per host (in current project).
// Mix of access to VMs (find all) and MIs (See: vm_ls() for "ingredients" of solution)
// The mic.HostREStr must be a pattern that captures the hostname part from the machine image as capture group 1
// e.g. export MI_HOSTPATT='^(\w+-\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3})'
// https://www.pulumi.com/registry/packages/gcp/api-docs/compute/machineimage/
func mi_time_stats() {
  //ctx := context.Background()
  //////// VMs //////////
  fmt.Fprintf(os.Stderr, "Proj: %s\n", mic.Project);
  vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF} //  
  err := vmc.Init()
  // Note: the flag.Parse() and Project reassign are workaround for inherent state problems for the
  // config -> env env.Set(cfg)-> CL flag.Parse() override seq.
  flag.Parse()
  vmc.Project = mic.Project;
  if err != nil { fmt.Fprintln(os.Stderr, "Failed to Init VMC: ", err); return; }
  fmt.Fprintf(os.Stderr, "Proj: %s\n", vmc.Project);
  allvms := vmc.GetAll()
  if len(allvms) < 1 { fmt.Println("No VMs found"); return; }
  stats := VMs.CreateStatMap(allvms) // Stats map - AT LEAST for tstats op/subcmd
  if vmc.Debug { fmt.Fprintf(os.Stderr, "%v %v", allvms, stats) }
  //////// MIs ///////////
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  all := mic.GetAll()
  // Because we collect stats by hostname, the pattern to match must be there !
  // NOTE: We *could* use source VM of the MI, gotten from MI info (and discard the whole HostREStr / HostRE)
  if mic.HostREStr == "" { fmt.Printf("Warning: No HostREStr in environment (MI_HOSTPATT) or config !"); return; }
  if mic.HostRE == nil   { fmt.Printf("Warning: No HostRE pattern matcher (RE syntax error ?) !"); return; }
  //totcnt := 0 // TODO: More diverse stats
  secnt := mic.HostRE.NumSubexp()
  fmt.Fprintf(os.Stderr, "Subexpressions: %d\n", secnt);
  if (secnt < 1) { fmt.Printf("Error: There are no RE captures for VM name - must have min. 1 !"); return; }
  // Gather stats by 1) correlating MI to a VM 2) Seeing if MI classifies as recent (<1week) or older (> 1week) - place this to stats of VM
  // cb - Add to stats[]-map
  cb := func(mi * computepb.MachineImage) { // mic MIs.CC
    t, err := mic.CtimeUTC(mi)
    if err != nil { fmt.Fprintf(os.Stderr, "MI C-TS not parsed !"); return; }
    agehrs := mic.AgeHours2(t)
    //if agehrs > float64(mic.KeepMaxH) { fmt.Printf("Too old\n"); continue; } // Do not discard, BUT ADD to stats
    //if (mic.HostRE != nil) { // No need to check, has been checked much earlier !!!
      
      // TODO: Extract Originating VM from mi instead of extracting with RE (?)
      // source_instance:"https://www.googleapis.com/compute/v1/projects/spgovusm1-saas-sec/zones/us-east4-a/instances/splkcm-10-252-8-138"
      miname := path.Base( mi.GetSourceInstance() )
      //fmt.Printf("%v", mi);
      //fmt.Printf("%s\n", miname); os.Exit(0) // Test
      /* Old legacy RE-based name extraction. Still do standard name detection ?
      m := mic.HostRE.FindStringSubmatch( mi.GetName() )
      // NOTE: HostRE is likely to ONLY match Std. name pattern, so no-match may/will happen for all the ad-hoc backups.
      if len(m) < 1 { fmt.Fprintf(os.Stderr, "Warning: No capture items for hostname matching (%s)\n", mi.GetName() ); return; }
      fmt.Fprintf(os.Stderr, "HOSTMatch: %v, MINAME: %s AGE: %f\n", m[1], mi.GetName(), agehrs );
      miname := m[1]
      mis, ok := stats[m[1]] // MI stat. Is this copy or ref to original ? https://golang.cafe/blog/how-to-check-if-a-map-contains-a-key-in-go
      */
      mis, ok := stats[miname]
      //if mis.Mincnt > 100 {} // Dummy
      if !ok { fmt.Fprintf(os.Stderr, "Warning: No stats entry for captured VM name '%s'\n", miname); return; } // m[1]
      
      if agehrs <= float64(mic.KeepMinH)  { mis.Mincnt++; //stats[m[1]].Mincnt++
      } else if agehrs <= float64(mic.KeepMaxH) { mis.Maxcnt++ ; //stats[m[1]].Maxcnt++
      } else {   mis.Delcnt++ } // Last: test/debug mis.Mincnt++;
    //}
    return
  }
  maxtime := false
  cb2 := func(mi * computepb.MachineImage) {
    t, err := mic.CtimeUTC(mi)
    if err != nil { fmt.Printf("MI C-TS not parsed !"); return; }
    agehrs := mic.AgeHours2(t)
    if (agehrs > float64( mic.KeepMaxH) ) { return; } // Test 1000000 => mic.KeepMaxH
    //
  }
  if maxtime { cb = cb2; }
  for _, mi := range all {
    //totcnt++
    
    cb(mi)
  }
  
  // The downside of having map value-struct as *pointer* is we see only mem-address in (raw Printf("%v", ...)) dump (!!!)
  //fmt.Printf("STATS: %v\n", stats)
  // Serialize/Sequentialize stats map to a slice/array (of per-vm stats) and the array further to JSON output.
  //TODO:
  
  vmmi_tstats_out(stats)
}

 // Output MI Statistics per timespan (stored in a map[vmname]{MIStat})
func vmmi_tstats_out (stats map[string]*VMs.MIStat) {
  //var reparr []*VMs.MIStat // Empty/0-items, no pre-alloc
  reparr := make([]*VMs.MIStat, len(stats)) // Prealloc to right size (Use indexes)
  i := 0
  for _, stat := range stats {
    //if fmt.Fprintf(os.Stderr, "%s %d %d\n", stat.Hostname, stat.Mincnt, stat.Maxcnt); // 2-statfields text version
    reparr[i] = stat // OLD: append(reparr, stat) // append() will append to current pre-alloc'd len of slice !!
    i++
  }
  jba, err := json.MarshalIndent(reparr, "", "  ") // ([]byte, error)
  if err != nil { fmt.Println("Error JSON-serializing VMMI stats !"); return; }
  fmt.Printf("%s\n", jba) // Ok w. []byte
  fmt.Fprintf(os.Stderr, "# %d Images from %s\n", len(reparr), mic.Project); //totcnt
}

// old: mi_del()


// old: mimilist_del_*
// old: mic_delete_mi


// Orig key_list

// List Projects (In Org, under all folders)
// For ansible GCP config see:
// - https://docs.ansible.com/ansible/latest/collections/google/cloud/gcp_compute_inventory.html
// - compose...: https://github.com/ansible/awx/issues/9812
func proj_list()  { // error ?
  flag.Parse()
  ans := Ansible;
  itemprefix := "";
  if ans { itemprefix = "  - " }
  //fmt.Printf("Got Filter: %s\n", Filter);
  //var qmap = map[string]string{} // {"include": "true"}
  qmap := GPs.KvParse(Filter) // Labelfilter
  qstr := GPs.Map2Query(qmap)
  if qstr != "" { fmt.Printf("Query: %s\n", qstr); }
  Projects := GPs.ProjectsList(qstr)
  fmt.Printf("# %d Projects retrieved by labelfilter: '%s'\n", len(Projects), qstr)
  if ans { fmt.Printf("---\nplugin: gcp_compute\nprojects:\n") }
  // Filter additionally by name ?
  for _, project := range Projects {
    // fmt.Printf("%s =>\n%v\n", project.ProjectId, *project)
    if itemprefix != "" { fmt.Print(itemprefix); }
    fmt.Printf("%s\n", project.ProjectId)
  }
  if ans {
    fmt.Printf("auth_kind: serviceaccount\nservice_account_file: %s\nhostnames:\n  - name\n# prefix: ...,key: ...\nkeyed_groups: []\n", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
    fmt.Printf("compose:\n  ansible_host: networkInterfaces[0].networkIP\n")
  }
  return
}
// List OR List + Backup VMs from all the projects discovered based on labels (See explanations on filtering below).
// E.g. 352 => 60 (5s)
// Note: --filter here is for the labels and must be on CLI in format "labels.include=true"
// See also: vm_backup
func projsvmbackup() {
  flag.Parse()
  qmap := GPs.KvParse(Filter) // Labelfilter
  qstr := GPs.Map2Query(qmap)
  //qstr := "" // "labels.include=true"
  if Suffix != "" { fmt.Printf("Current suffix: %s\n", Suffix); }
  var vmc VMs.CC = VMs.CC{Project: ""} // CredF: ""
  err := vmc.Init();
  if err != nil { fmt.Printf("No vmc: %v\n", err); return; }
  Projects := GPs.ProjectsList(qstr)
  if Projects == nil { fmt.Printf("No Projects Listed"); return; }
  pvms := GPs.ProjectsVMs(Projects, vmc)
  //mic := MIs.CC{Project: ""} // use module-global
  mic.Init(); // MUST init (not inited yet)
  tnow := mic.Tnow()
  usesuff := mic.DatePrefix(Suffix,  &tnow ) // mic.tnow not directly accessible - use mic.Tnow(). time.Now().UTC()
  fmt.Printf("It is now (UTC): %s. Another way: %s\n", mic.Tnow(),  (&tnow).Format("2006-01-02") );
  fmt.Printf("Pass altsuff: '%s'\n", usesuff)
  all := []*computepb.Instance{}
  // Initially: list + append(all)
  for _, pvm := range pvms {
    fmt.Printf("  - %s / %s\n", pvm.Project.ProjectId, pvm.Vm.GetName());
    //err = mic.CreateFrom(pvm.Vm, usesuff, "")
    //if err !=nil { fmt.Printf("Error Creating MI out of VM: '%s': %s\n", pvm.Vm.GetName(), err); continue; }
    //fmt.Println("MI created w. suffix '%s' from VM '%s' successfully\n", usesuff, pvm.Vm.GetName())
    all = append(all, pvm.Vm)
  }
  if (gop == "projsvmlist") { return; } // List only, do NOT backup (return early)
  // Backup (Bulk)
  mic.CreateFromMany(all, nil, usesuff)
}
func load_org_conf(fn string) *OrgTree.OrgEnt {
  // Problem: in OrgEnt we have generic json member "id", not fit for config file.
  // Solution: create a situation specific struct matching json mems, populate it, do 2-mem-copy
  type OrgConf struct { OrgId string } // Dummy struct to ONLY hold the OrgId
  var oc OrgConf = OrgConf{}
  //root := &OrgEnt{ Name: orgname, Id: orgid, Etype: "organization" };
  // Run unsmarshal on fn
  _, err := os.Stat(fn)
  if err != nil { return nil }
  cont, err := os.ReadFile(fn)
  if err != nil { return nil }
  err = json.Unmarshal(cont, &oc)
  // NOT: return &oc
  if oc.OrgId != "" {
    on := OrgTree.OrgEnt{Id: oc.OrgId, Etype: "organization"}
    return &on
  }
  return nil
}
func orgtree_dump() {
  oname := "my.org";
  oid := "007"
  if os.Getenv("ORGID") != "" { oid = os.Getenv("ORGID"); }
  var oload = OrgTree.OrgLoader{};
  oload.LoadInit()
  oload.Debug = true
  root := OrgTree.NewOrgTree(oid, oname);
  root2 := load_org_conf("./goclowdy.conf.json") // Try config
  if root2 != nil { root = root2 }
  //fmt.Printf("Constructed Org, but skipping traverse\n"); return; // DEBUG
  oload.LoadOrgTree(root);
  dump, err := json.MarshalIndent(root, "", "  ")
  if err != nil { fmt.Printf("Error serializing to JSON: %s\n", err); }
  if dump == nil { return }
  fmt.Printf("%s\n",dump); // "Done main: %s\n", 
  //fmt.Println(" ",dump);
  root.Process(OrgTree.Dumpent, nil)
  return
}
// Shut down project VM:s (or all under folder ?)
// TODO: label filter variant of vmset_filter()
func proj_shutdown() {
  flag.Parse() // parse for --filter
  vmc := VMs.CC{Project: mic.Project, CredF: mic.CredF} //  
  err := vmc.Init()
  if err != nil { fmt.Println("Failed to Init VMC: ", err); return; }
  all := vmc.GetAll() //; fmt.Println(all)
  icnt := len(all)
  if icnt == 0 { fmt.Println("No VMs found"); return }
  fmt.Printf("# Got %v  Instances (from project '%s')\n", icnt, vmc.Project) // Initial ... (Filtering ..)
  all = vmset_filter(all, Filter); //  Allow filter
  //stats := VMs.CreateStatMap(all)
  //fmt.Printf("%v", stats)
  // Test for daily MI. This is now embedded to mic.CreateFrom() logic.
  //mic.Init() // If bottom MI lookup enabled
  for _, it := range all{ // Instance
    vmc.Stop(it)
  }
}


///////////// Tentative - domain specific dir tree filtering / listng //////////////////////////////
// Needs: import (
// "io/fs" // for DirEntry present in WalkDir cb signatures
// "path/filepath" // Walk / WalkDir and  abs-to-rel by Rel()
// "bytes" // Fors splitting bytearrays bytes.Split()
//)
/*
// Match individual lines in bytearray (must be split inside)
type LineMatch struct { lineno int; mline []byte; linecnt int};
func linematch(cont []byte) []LineMatch { // [][]byte - cannot capture line number
  if cont == nil { return nil; }
  //mlines := [][]byte{};
  matches := []LineMatch{};
  //if lines == nil {}
  lines := bytes.Split(cont, []byte("\n"))
  for idx, l := range lines { // idx
    //fmt.Printf("%s\n", string(l))
    // Match
    matched := FilterRE.Match(l)
    if (matched) {
      //fmt.Printf("Line %d: %s\n", idx, l);
      //mlines = append(mlines, l);
      matches = append(matches, LineMatch{lineno: idx+1, mline: l, linecnt: len(lines)});
    }
  }
  if len(matches) < 1 { return nil; }
  return matches
}

// Note: Alternative path/filepath.WalkFunc would be called w. FileInfo
// https://pkg.go.dev/io/fs#DirEntry
func procfsnode_de(path string, d fs.DirEntry, err error) error {
  // How to prevent diving into dir (e.g. by dir name)
  if d.IsDir() {
    //if excludes[ d.Name() ] { return  filepath.SkipDir; }
    if d.Name() == ".git" { return  filepath.SkipDir; }
    //fmt.Printf("Directory: %s (Name: %s)\n", path, d.Name());
    return nil;
  }
  //if Debug { fmt.Printf("File: %s (Name: %s)\n", path, d.Name()); }
  // 
  if d.Name() != "index.html" { return nil; } // e.g. package.json, index.html, terragrunt.hcl
  // if strings.HasPrefix(d.Name(), Prefix) { fmt.Printf("Has prefix %s\n", ); return nil; }
  // https://pkg.go.dev/io/fs#FileInfo
  fi, err := d.Info()
  if err != nil { fmt.Printf("No file info for %s\n", d.Name()); return nil; }
  if fi == nil  { return nil; }
  cont, err := os.ReadFile(path)
  if err != nil { fmt.Printf("Error reading file %s\n", path); return nil; }
  if len(cont) == 0 { fmt.Printf("Empty file: %s\n", path); return nil; }
  relpath, err := filepath.Rel(pathroot, path)
  if err != nil { relpath = path; }
  //matched, _ := regexp.Match(Filter, cont) // MatchString()
  //matched := FilterRE.Match(cont) // MatchString() for str
  //if (matched) { fmt.Printf("Match in %s !!!\n", path); }
  lines := bytes.Split(cont, []byte("\n"))
  matches := linematch(cont);
  
  
  //if err != nil { fmt.Printf("Error matching mod."); return nil; }
  //if matched { fmt.Printf("Matched: `%s`\n", matched); }
  // mre := *regexp.MustCompile("([a-z]+-[\\w-]+)");
  // new String(bytes, StandardCharsets.UTF_8)
  // p = make(map[string]string); for i, name := range compRegEx.SubexpNames() { p[name] = match[i] } (for i > 0 && i <= len(match) )
  caps := ModRe.FindSubmatch(cont) // NA: FindSubmatch Use: FindStringSubmatch
  signifier := []byte("NONE");
  //if err != nil    { fmt.Printf("Error searching for mod."); return nil; }
  if len(caps) > 0 { signifier = caps[1]  } // fmt.Printf("No mod. fnd in %s\n", path); return nil;
  if matches == nil {  return nil; } // fmt.Printf("  - No match in %s !\n", relpath);
  fmt.Printf("%s (%d l.) => module: %s\n", relpath, len(lines), signifier); // caps[1]
  if NoOut { return nil; } // No match output
  if matches != nil {
    for _, m := range matches { fmt.Printf("%d: %s\n", m.lineno, m.mline); }
  }
  return nil;
}

var ModRe regexp.Regexp; // Signifier
var pathroot string = ".";
var NoOut = false;
// var ExcludeMap =  make(map[string]bool);
// Grep from a tree
// Use global arg --filter to filter files
// Get path from flag arg 0
// https://pkg.go.dev/path/filepath
// https://stackoverflow.com/questions/30483652/how-to-get-capturing-group-functionality-in-go-regular-expressions
// TODO: - Option for no matches (take from grep)
func file_filter() {
  flag.BoolVar(&NoOut,     "no-out", false, "No match output (with line number and match).")
  flag.Parse()
  // err = json.Unmarshal(cont, &dsgrepconf) // Config ?
  if Filter == "" { fmt.Printf("Must pass --filter to do grep-filtering.\n"); return; } // flag.Arg(0) ?
  FilterRE = *regexp.MustCompile(Filter) // Always custom filter from CLI
  //if (DSGREP_MODRE == "") { fmt.Printf("No module re set (e.g. in Env. !"); return; }
  ModRe = *regexp.MustCompile("([a-z]+-[\\w-]+)") // TODO: Should come From cfg
  //if ModRE
  //pathroot := "."; // now mod-global
  // flag.Arg(0) &&
  if  (flag.Arg(0) != "") { pathroot = flag.Arg(0); } // Arg(1) like grep ?
  // https://pkg.go.dev/io/fs#WalkDirFunc
  filepath.WalkDir(pathroot, procfsnode_de);
  // 
}
*/
// Role-diff 2 roles (in few different ways) in a project context
//func iamrolecmp () {}
