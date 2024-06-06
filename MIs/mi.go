package MIs

import (
	"context"
	"fmt" // for name
	"os"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"

	//"strings" // ??
	"errors"
	"regexp"
	"strconv" // Atoi()

	"flag"

	"github.com/codingconcepts/env"
	"github.com/ohollmen/goclowdy/VMs"
)

// Machine Image Client Config
type CC struct {
  Project string  `env:"GCP_PROJECT"`
  CredF string    `env:"GOOGLE_APPLICATION_CREDENTIALS"`
  //TZn string      `env:"GCP_CLOCK_TZN"`
  WD_keep int
  MD_keep int
  KeepMinH int // dur_keep_all => KeepMinH
  KeepMaxH int
  // Note: Priv
  tnow time.Time // Time (at start of process(ing)) to share within an action
  //Tloc * time.Location
  StorLoc string // e.g. zone or "us"
  // ctx !!
  c * compute.MachineImagesClient
  DelOK bool       `env:"MI_DELETE_EXEC"`
  Debug bool
  NameREStr string `env:"MI_STDNAME"` // MI Standard name RE
  NameRE * regexp.Regexp    // Use func (*Regexp) String to get orig string
  HostREStr string `env:"MI_HOSTPATT"` // Hostname capture from MI name (should have 2 cap.parens 1sh: hostname, 2nd: date)
  HostRE * regexp.Regexp
  ChunkDelSize int  `env:"MI_CHUNK_DEL_SIZE"`
  WorkerLimit int
  Wg * sync.WaitGroup // Do not init, use only when context requires
}


const (
  KEEP_SAFE int = 0
  KEEP_NEW = 1 // Newer than KeepMinH
  KEEP_WD = 2 // In intermediate time, Matching WD_keep
  DEL_1W  = 3 // In intermediate time, not WD_keep
  DEL_OLD  = 4 // Older than KeepMaxH
  KEEP_CUSTOM = 5 // Custom named MI
  KEEP_MD = 6 // Day of the month
)
const (TAKE_NONE uint8 = 0; TAKE_DAILY = 1; TAKE_WEEKLY = 2; TAKE_MONTHLY = 4)
func (cfg * CC) Client() * compute.MachineImagesClient {
  return cfg.c
}
func (cfg * CC) Tnow() time.Time {
  return cfg.tnow;
}
// Apply non-empty env. vars to config members.
func (cfg * CC) EnvMerge() {
  if os.Getenv("GCP_PROJECT") != "" { cfg.Project = os.Getenv("GCP_PROJECT") }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" { cfg.CredF = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") }
  // if os.Getenv("GCP_CLOCK_TZN") != "" { cfg.TZn = os.Getenv("GCP_CLOCK_TZN") }
  if os.Getenv("MI_STDNAME") != "" { cfg.NameREStr = os.Getenv("MI_STDNAME") }
  if os.Getenv("MI_CHUNK_DEL_SIZE") != "" { cfg.ChunkDelSize, _ = strconv.Atoi( os.Getenv("MI_CHUNK_DEL_SIZE") ); }
  if os.Getenv("MI_DELETE_EXEC") != "" { cfg.DelOK = true; } // Any non-empty
}
func (cfg * CC) Validate() error {
  if cfg.Project == "" { return errors.New("No GCP Project !") }
  if cfg.CredF == "" { return errors.New("No GCP App Credentials file !") }
  // Part of validation ?
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
    fmt.Printf("Setting G-A-C=%s\n", cfg.CredF);
    os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.CredF)
  }
  return nil
}
// Cons: time.ParseDuration("10s") from https://go.dev/blog/package-names
func (cfg * CC) Init() int { // TODDO pass: clpara map[string]string to patch after env
  ctx := context.Background()
  //cfg.EnvMerge() // explicit implementation
  // Note: This (w. `env:...` tags seems to handle the type conversions (even bool value 0)
  err := env.Set(cfg)
  if err != nil { fmt.Println("Error setting config mems from env !"); return 1  }
  // Parse CL agcs right after (this sets the timing of parsing)
  flag.Parse()
  // CL params from map ?
  //if len(clpara): { // OR: clpara
  //  if clpara["project"]  != "" { cfg.Project = clpara["project"]; }
  //  if clpara["appcreds"] != "" { cfg.CredF = clpara["appcreds"]; } // CredF
  //}
  // NEW: Default to UTC
  //if cfg.TZn == "" { cfg.TZn = "Europe/London" }
  //cfg.Tloc, err = time.LoadLocation(cfg.TZn)
  //if err != nil { fmt.Println("Error Loading time.Location !"); return 1 }
  //cfg.tnow = time.Now() // Now=Local OLD
  cfg.tnow = time.Now().UTC()
  // Note/Investigate: Setting G_A_C here is effective, but not in Validate() !?
  os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.CredF)
  // Init client ?
  // var err error
  cfg.c, err = compute.NewMachineImagesRESTClient(ctx)
  if err != nil { fmt.Println("Failed to init GCP MI Rest Client: ", err); return 1 }
  if cfg.StorLoc == "" { cfg.StorLoc = "us"; }
  // Good Defaults for KeepMinH, KeepMaxH
  if cfg.KeepMinH < 1 { cfg.KeepMinH = 168; }
  if cfg.KeepMaxH < 1 { cfg.KeepMaxH = (24 * (548 + 7)); }
  // Naming
  // https://pkg.go.dev/regexp
  //var stdnamere * regexp.Regexp // Regexp
  //var err error
  // E.g. "^\\w+-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}$" (in Go runtime)
  // E.g. "^[a-z0-9-]+?-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{1,3}-\\d{4}-\\d{2}-\\d{2}$" // No need for 1) \\ before [..-] 2) \ before [ / ]
  //stdm := stdnamere.MatchString( "myhost-00-00-00-00-1900-01-01" ); // Also reg.MatchString() reg.FindString() []byte()
  //if !stdm { fmt.Println("STD Name re not matching "); return }
  if cfg.NameREStr != "" {
    cfg.NameRE, err = regexp.Compile(cfg.NameREStr) // (*Regexp, error) // Also MustCompile
    if err != nil { fmt.Println("Cannot compile STD name RegExp"); return 1 }
  }
  if cfg.HostREStr != "" {
    cfg.HostRE, err = regexp.Compile(cfg.HostREStr)
    if err != nil { fmt.Println("Cannot compile hostpatt RegExp"); return 1 }
  }
  // 
  err = cfg.Validate()
  if err != nil { fmt.Println("Config Validation Failed. Please check your JSON config, Environment vars and CL params."); return 1 }
  return 0
}
// Legacy ParseInLocation way of detecting age.
func (cfg * CC) AgeHoursXX(mi * computepb.MachineImage) float64 { // int ?
  // See elaboration in Classify
  fmt.Println("WARNING: Using legacy Location based Age computation !!!")
  //t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.Tloc)
  t, err := time.Parse(time.RFC3339, mi.GetCreationTimestamp())
  t = t.UTC()
  if err != nil { return -1; }
  nd := cfg.tnow.Sub(t) // Duration/Age
  return nd.Hours(); // cast int ? round ?
}
// New way of computing age. Call CtimeUTC to get started, call this with results.
func (cfg * CC) AgeHours2(t  time.Time) float64 { // int ?
  // See elaboration in Classify
  //t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.Tloc)
  // t, err := time.Parse(time.RFC3339, mi.GetCreationTimestamp()).UTC()
  //if err != nil { return -1; }
  nd := cfg.tnow.Sub(t) // Duration/Age
  return nd.Hours() // cast int ? round ?
}
// Create UTC time.Time (no zone) from MI CreationTimestamp (which always has explicit TZ).
// After parsing time.Time gets convertede to UTC (w. no TZ).
func (cfg * CC)CtimeUTC(mi * computepb.MachineImage) (time.Time, error) {
  //t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.Tloc)
  t, err := time.Parse(time.RFC3339, mi.GetCreationTimestamp());
  if err != nil { return t, err; }
  t = t.UTC()
  return t, nil;
}
// Classify an MI for keep / delete (and the classifying reason for it).
func (cfg * CC) Classify(mi * computepb.MachineImage) int {
  // Since GCP TS always has TZ spec, drop cfg.Tloc (not effecive) and call time.Parse(...).UTC()
  //t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.Tloc) // Def. UTC
  //t, err := time.Parse(time.RFC3339, mi.GetCreationTimestamp())
  t, err := cfg.CtimeUTC(mi)
  if err != nil { return KEEP_SAFE; } // Parse error (play safe, KEEP)
  //ALREADY:t = t.UTC()
  //time.Time; time.Location; t.Lo
  //nd := cfg.tnow.Sub(t) // subtract (now-t) for Duration/Age
  //hrs := nd.Hours()
  hrs := cfg.AgeHours2(t) // TODO !!! (Need above for err or check for ret'd -1 / err ?)
  //if hrs < 0 { return KEEP_SAFE; } // T-Parse error in AgeHours()
  // NEW: Non-std Naming
  //if (cfg.NameRE != nil) && (!cfg.NameRE.MatchString( mi.GetName() )) { return KEEP_CUSTOM; }
  // NEW: Use cfg.KeepMaxH (OLD: (24 * (365 + 7))). 
  if hrs > float64(cfg.KeepMaxH) { return DEL_OLD; } // fmt.Println("DEL "); // Need float64() ?
  // Less than MAX period (e.g. 1Y+1W) old ... but
  // Test for always keep-period
  // float64
  if hrs < float64(cfg.KeepMinH)     {  return KEEP_NEW; } // fmt.Println("KEEP (new, < week old)");
  if cfg.WD_keep == int(t.Weekday()) {  return KEEP_WD; } // fmt.Println("KEEP correct day");
  if cfg.MD_keep == int(t.Day())     {  return KEEP_MD; } // Day of the month
  //fmt.Println(nd)
  return DEL_1W
}

// To delete .. based on keep-classification. All DEL_* enums should be included here.
func ToBeDeleted(cl int) bool {
  if (cl == DEL_1W) || (cl == DEL_OLD) { return true } // All DEL_* enums
  return false
}

// Figure out which kinds machine images to take (daily=1,weekly=2,monthly=4 to pack 3 bits into an uint) for current day.
// Use the const enums to test the results (e.g.):
// ```
// totake := mic.MIsToTake()
// if totake | TAKE_WEEKLY { ... do weekly ... }
// ````
func (cfg * CC) MIsToTake(t * time.Time) uint8 { // t * time.Time ?
  if t == nil { t = &cfg.tnow; } // time.Now().UTC()
  var totake uint8 = TAKE_DAILY; // Always
  if cfg.WD_keep == int(t.Weekday()) { totake |= TAKE_WEEKLY; }
  if cfg.MD_keep == int(t.Day())     { totake |= TAKE_MONTHLY; }
  return totake;
}
// Convert totake-bits to slice of timeunit-suffixes to use for MI creation.
func BitsToTimesuffix(bits uint8) []string {
  sfs := []string{}
  //bvs := [uint8]{1,2,4}
  //for _, bv := range bvs { if bv{} }
  if (bits & 1) > 0 { sfs = append(sfs, "daily"); }
  if (bits & 2) > 0 { sfs = append(sfs, "weekly"); }
  if (bits & 4) > 0 { sfs = append(sfs, "monthly"); }
  return sfs
}

// Delete a MI (passed by name, in the project of mic-client).
// Alt way:
// delimg := computeService.MachineImages.Delete(projid, miname)
// _, err := delimg.Do()
//OLD: func (cfg * CC) Delete(mi * computepb.MachineImage) error { ... now kep more loosely coupled (by name)
func (cfg * CC) Delete(miname string) error {
  ctx := context.Background()
  fmt.Println("Should delete "+ miname); // mi.GetName()
  if ! cfg.DelOK { fmt.Printf("Not Deleting, DelOK=%v", cfg.DelOK); return nil; } // return was not there
  // Prepare request
  dreq := &computepb.DeleteMachineImageRequest{MachineImage: miname, Project: cfg.Project} //RequestId  mi.GetName()
  //dreq.Reset()
  // Call clinet (c) to *actually* delete
  op, err := cfg.c.Delete(ctx, dreq)
  if err != nil { fmt.Printf("Failed to delete MI: %s (%v) ", miname, err); return err; } // mi.GetName()
  // if cfg.ChunkDelSize == 0 ???
  err = op.Wait(ctx)
  if err != nil { fmt.Println("Error waiting for MI Deletion of ", miname); return err; } // mi.GetName()
  fmt.Println("Success deleting MI: ", miname) // mi.GetName()
  return nil
}
// Prefix time (as ISO date) to existing suffi in a "MI name friedly" way (Using '-')
func (cfg * CC) DatePrefix(suff string, t * time.Time ) string {
  if t == nil { t = &cfg.tnow; }
  if suff == "" { return t.Format("2006-01-02"); }
  return t.Format("2006-01-02") + "-" + suff
}
// Create MI from a VM (passed as param) with optional custom name suffix.
// Default MI name will be VM name with "-" + ISO date
// inst * computepb.Instance (Instead of mini * string, make variadic ?)
// options: force is in effect by cfg.DelOK. If mic.Wg (work group) is set, auto-defers mic.Wg.Done()
func (cfg * CC) CreateFrom(inst * computepb.Instance, altsuff string, Project string) error { // Project string
  
  var storageLocation []string
  storageLocation = append(storageLocation, cfg.StorLoc) // "us"
  var imgname string
  // Figure out MI name
  if altsuff == "" { imgname = StdName(inst.GetName()) // Create STD name HN + "-" + ISO
  } else { imgname = inst.GetName() + "-" + altsuff }
  if cfg.Debug { fmt.Printf("MI name to use: '%s', storloc: '%s'\n", imgname, storageLocation[0]); }
  if cfg.Wg != nil { fmt.Printf("Got Wg - defer...\n"); defer cfg.Wg.Done(); }
  // Check existing. This is good no matter what for clarity and possibly avoiding creation call
  mi := cfg.GetOne(imgname)   // , cfg.c
  if mi != nil  {
    fmt.Println("Found (name-overlapping) MI: ", imgname)
    // Skip (What to return)
    if cfg.DelOK == false { fmt.Printf("Skipping MI creation for '%s' becase MI exists and no Forcing is on.\n", imgname); return nil; }
    // or Delete (on DelOK/force)
    err := cfg.Delete(mi.GetName())
    if err != nil { fmt.Printf("Tried deleting MI with overlapping name (%s), but failed: %v\n", imgname, err); return err; }
  } else {
    fmt.Println("No overlaps Found for MI-name: ", imgname)
  }
  //NONEED:instance_path := cfg.instpath(inst) // pass: Instance
  instance_path := inst.GetSelfLink()
  fmt.Printf("Instance-path: %s\n", instance_path)
  // Orig examples propose closing of client here. is that necessary ? Answer: NO - there is no concurrency problems w. client.
  // This would imply a full re-instantiation / re-config of client
  // Likely because client is established in the orig. func scope.
  //defer cfg.c.Close()
  var UseProject string = VMs.VMProj(inst)
  if UseProject == "" { UseProject = Project; }
  req := &computepb.InsertMachineImageRequest{
    Project: UseProject, // cfg.Project,
    SourceInstance: &instance_path,
    MachineImageResource: &computepb.MachineImage{
      Name: &imgname,
      SourceInstance: &instance_path,
      StorageLocations: storageLocation,
      // TODO: nil or inst of...
      // MachineImageEncryptionKey: &computepb.CustomerEncryptionKey{ KmsKeyName: &kkn, }, // Key name
    },
  }
  // Do we need to share exact context with the caller ? A: No need to do that.
  ctx := context.Background() // DO we have to use shared context, e.g. mic.Ctx OR mic.?
  op, err := cfg.c.Insert(ctx, req)
  // instancename := inst.GetName() // mini
  if err != nil {
    fmt.Printf("Error in c.Insert() - MI creation failed for instance '%s': %v\n", inst.GetName() , err)
    return(err)
  }
  err = op.Wait(ctx)
  if err != nil {
    fmt.Printf("Error in op.Wait(ctx) - MI Creation failed for instance %s: %v\n", inst.GetName() , err)
    return(err)
  }
  fmt.Println("Success creating MI: ", imgname) // on debug
  return nil
}

// OLD: , c * compute.MachineImagesClient
func (cfg * CC) GetOne(in string) *computepb.MachineImage {
  ctx := context.Background()
  req := &computepb.GetMachineImageRequest{MachineImage: in, Project: cfg.Project}
  // https://pkg.go.dev/cloud.google.com/go/compute/apiv1#MachineImagesClient.Get
  mi, err := cfg.c.Get(ctx, req)
  if err != nil { fmt.Printf("Error fetching MI: %s %v\n", in, err); return nil }
  if cfg.Debug {  fmt.Printf("Got mi: %s\n", mi.GetName() ); }
  return mi
}
// New do-it-all search. Client MUST be inited before.
func (mic * CC) GetAll() []*computepb.MachineImage {
  ctx := context.Background()
  var arr []*computepb.MachineImage  // MIMI
  var maxr uint32 = 500 // 20
  if mic.Project == "" { fmt.Println("No Project passed"); return nil }
  req := &computepb.ListMachineImagesRequest{
    Project: mic.Project,
    MaxResults: &maxr } // Filter: &mifilter } // 
  if req == nil { return nil }
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No mi:s from "+mic.Project); return nil }
  totcnt := 0
  for {
    //fmt.Println("Next ..."); // DEBUG
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break }
    if mi == nil { fmt.Println("No mi gotten in iteration. check (actual) creds, project etc."); break }
    arr = append(arr, mi)
    totcnt++
  }
  return arr
}
// ...StdName(i.GetName()). TODO: Opt date
func StdName(mn string) string {
  now := time.Now()
  y, m, d := now.Date()
  dtstr := fmt.Sprintf("%d-%.2d-%.2d",y, int(m), d)
  return mn+"-"+dtstr
}
// Does VM have this ? top-level: self_link
//func (cfg * CC) instpath(inst *computepb.Instance) string { // project string
//  // OLD: project
//  return fmt.Sprintf( "projects/%s/zones/%s/instances/%s", cfg.Project, InstZone(inst), inst.GetName() )
//}
// TODO: Inst meth
//func InstZone(inst *computepb.Instance) string {
//  fullzone := inst.GetZone()
//  li := strings.LastIndex(fullzone, "/")
//  return fullzone[li+1:len(fullzone)]
//}
