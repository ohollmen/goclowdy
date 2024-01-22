package MIs

import (
	"context"
	"fmt" // for name
	"os"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"

	//"strings" // ??
	"errors"
	"regexp"
	"strconv" // Atoi()
)

// Machine Image Client Config
type CC struct {
  Project string
  CredF string
  TZn string
  WD_keep int
  KeepMinH int // dur_keep_all => KeepMinH
  KeepMaxH int
  // Priv
  tnow time.Time
  Tloc * time.Location
  StorLoc string
  //tnow Time
  // ctx !!
  c * compute.MachineImagesClient
  DelOK bool
  Debug bool
  NameREStr string
  NameRE * regexp.Regexp    // Use func (*Regexp) String to get orig string
  ChunkDelSize int
  WorkerLimit int
}


const (
  KEEP_SAFE int = 0
  KEEP_NEW = 1 // Newer than KeepMinH
  KEEP_WD = 2 // In intermediate time, Matching WD_keep
  DEL_1W  = 3 // In intermediate time, not WD_keep
  DEL_OLD  = 4 // Older than KeepMaxH
  KEEP_CUSTOM = 5 // Custom named MI
)

func (cfg * CC) Client() * compute.MachineImagesClient {
  return cfg.c
}

func (cfg * CC) EnvMerge() {
  if os.Getenv("GCP_PROJECT") != "" { cfg.Project = os.Getenv("GCP_PROJECT") }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" { cfg.CredF = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") }
  if os.Getenv("GCP_CLOCK_TZN") != "" { cfg.TZn = os.Getenv("GCP_CLOCK_TZN") }
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
func (cfg * CC) Init() int {
  ctx := context.Background()
  cfg.EnvMerge()
  // Default to UTC
  if cfg.TZn == "" { cfg.TZn = "Europe/London" }
  cfg.Tloc, _ = time.LoadLocation(cfg.TZn)
  cfg.tnow = time.Now() // Now=Local
  // Note/Investigate: Setting G_A_C here is effective, but not in Validate() !?
  os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.CredF)
  // Init client ?
  var err error
  cfg.c, err = compute.NewMachineImagesRESTClient(ctx)
  if err != nil { fmt.Println("Failed to init GCP MI Rest Client: ", err); return 1 }
  if cfg.StorLoc == "" { cfg.StorLoc = "us"; }
  // Good Defaults for KeepMinH, KeepMaxH
  if cfg.KeepMinH < 1 { cfg.KeepMinH = 168; }
  if cfg.KeepMaxH < 1 { cfg.KeepMaxH = (24 * (365 + 7)); }
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
  // 
  err = cfg.Validate()
  if err != nil { fmt.Println("Config Validation Failed. Please check your JSON config, Environment vars and CL params."); return 1 }
  return 0
}

func (cfg * CC) Classify(mi * computepb.MachineImage) int {
  t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.Tloc) // Def. UTC
  nd := cfg.tnow.Sub(t) // Duration/Age
  if err != nil { return KEEP_SAFE; }
  hrs := nd.Hours()
  // NEW: Non-std Naming
  if (cfg.NameRE != nil) && (!cfg.NameRE.MatchString( mi.GetName() )) { return KEEP_CUSTOM; }
  // NEW: Use cfg.KeepMaxH (OLD: (24 * (365 + 7)))
  if hrs > float64(cfg.KeepMaxH) { return DEL_OLD; } // fmt.Println("DEL "); // Need float64() ?
  // Less than MAX period (e.g. 1Y+1W) old ... but
  // Test for always keep-period
  // float64
  if hrs < float64(cfg.KeepMinH) {  return KEEP_NEW; } // fmt.Println("KEEP (new, < week old)");
  if cfg.WD_keep == int(t.Weekday()) {  return KEEP_WD; } // fmt.Println("KEEP correct day");
  
  //fmt.Println(nd)
  return DEL_1W
}

// To delete .. based on keep-classification.
func ToBeDeleted(cl int) bool {
  if (cl == DEL_1W) || (cl == DEL_OLD) { return true }
  return false
}

// Old sign: mi * computepb.MachineImage
// Alt way:
// delimg := computeService.MachineImages.Delete(projid, miname)
// _, err := delimg.Do()
//func (cfg * CC) Delete(mi * computepb.MachineImage) error { // , c * compute.MachineImagesClient
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

// inst * computepb.Instance (Instead of mini, make variadic ?)
// OLD: mini * string
// TODO: Allow options: altsuff, force
func (cfg * CC) CreateFrom(inst * computepb.Instance, altsuff string) error { // , cfg * CC c * compute.MachineImagesClient
  
  var storageLocation []string
  storageLocation = append(storageLocation, cfg.StorLoc) // "us"
  var imgname string
  // Figure out MI name
  if altsuff == "" { imgname = StdName(inst.GetName())
  } else { imgname = inst.GetName() + "-" + altsuff }
  // Check existing. This is good no matter what for clarity and possibly avoiding creation call
  mi := cfg.GetOne(imgname)   // , cfg.c
  if mi != nil  {
    fmt.Println("Found (name-overlapping) MI: ", imgname)
    // Skip (What to return)
    if cfg.DelOK == false { fmt.Println("Skipping MI creation becase MI exists and no Forcing is on."); return nil; }
    // or Delete (on DelOK/force)
    err := cfg.Delete(mi.GetName())
    if err != nil { fmt.Printf("Tried deleting MI with overlapping name (%s), but failed: %v\n", imgname, err); return err; }
  }
  //NONEED:instance_path := cfg.instpath(inst) // pass: Instance
  instance_path := inst.GetSelfLink()
  // Orig examples propose closing of client here. is that necessary ?
  // This would imply a full re-instantiation / re-config of client
  // Likely because client is established in the orig. func scope.
  //defer cfg.c.Close()
  req := &computepb.InsertMachineImageRequest{
    Project: cfg.Project,
    SourceInstance: &instance_path,
    MachineImageResource: &computepb.MachineImage{
      Name: &imgname,
      SourceInstance: &instance_path,
      StorageLocations: storageLocation,
      // TODO: nil or inst of...
      // MachineImageEncryptionKey: &computepb.CustomerEncryptionKey{ KmsKeyName: &kkn, }, // Key name
    },
  }
  ctx := context.Background()
  op, err := cfg.c.Insert(ctx, req) // cfg.c
  // instancename := inst.GetName() // mini
  if err != nil {
    fmt.Printf("Error in Insert - vm image failed for instance %s - %v\n", inst.GetName() , err)
    return(err)
  }
  err = op.Wait(ctx)
  if err != nil {
    fmt.Printf("Error in Wait(ctx) - vm image failed for instance %s - %v\n", inst.GetName() , err)
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
  if err != nil { fmt.Println("Error fetching ", in); return nil }
  return mi
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
