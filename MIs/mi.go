package MIs

import (
  compute "cloud.google.com/go/compute/apiv1"
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  "time"
  "os"
  "fmt" // for name
  "context"
  //"strings" // ??
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
  tloc * time.Location
  StorLoc string
  //tnow Time
  // ctx !!
  c * compute.MachineImagesClient
  DelOK bool
  Debug bool
}


const (
  KEEP_SAFE int = 0
  KEEP_NEW = 1 // Newer than KeepMinH
  KEEP_WD = 2 // In intermediate time, Matching WD_keep
  DEL_1W  = 3 // In intermediate time, not WD_keep
  DEL_OLD  = 4 // Older than KeepMaxH
)

func (cfg * CC) Client() * compute.MachineImagesClient {
  return cfg.c
}
// Cons: time.ParseDuration("10s") from https://go.dev/blog/package-names
func (cfg * CC) Init() int {
  ctx := context.Background()
  // Default to UTC
  if os.Getenv("GCP_CLOCK_TZN") != "" { cfg.TZn = "Europe/London" }
  cfg.tloc, _ = time.LoadLocation(cfg.TZn)
  cfg.tnow = time.Now() // Now=Local
  if os.Getenv("GCP_PROJECT") != "" { cfg.Project = os.Getenv("GCP_PROJECT") }
  if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" { cfg.CredF = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") }
  // Init client ?
  var err error
  cfg.c, err = compute.NewMachineImagesRESTClient(ctx)
  if err != nil { return 1 }
  if cfg.StorLoc == "" { cfg.StorLoc = "us"; }
  //OLD: cfg.DelOK = false
  // Good Defaults for KeepMinH, KeepMaxH
  if cfg.KeepMinH < 1 { cfg.KeepMinH = 168; }
  if cfg.KeepMaxH < 1 { cfg.KeepMaxH = (24 * (365 + 7)); }
  return 0
}

func (cfg * CC) Classify(mi * computepb.MachineImage) int {
  t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.tloc) // Def. UTC
  nd := cfg.tnow.Sub(t) // Duration/Age
  if err != nil { return KEEP_SAFE; }
  hrs := nd.Hours()
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

// 
func (cfg * CC) Delete(mi * computepb.MachineImage) error { // , c * compute.MachineImagesClient
  ctx := context.Background()
  fmt.Println("Should delete "+ mi.GetName());
  if ! cfg.DelOK { fmt.Printf("Not Deleting, DelOK=%v", cfg.DelOK); }
  // Prepare request
  dreq := &computepb.DeleteMachineImageRequest{MachineImage: mi.GetName(), Project: cfg.Project} //RequestId
  //dreq.Reset()
  // Call clinet (c) to *actually* delete
  op, err := cfg.c.Delete(ctx, dreq)
  if err != nil { fmt.Printf("Failed to delete MI: %s (%v) ", mi.GetName(), err); return err; }
  err = op.Wait(ctx)
  if err != nil { fmt.Println("Error waiting for MI Deletion of ", mi.GetName()); return err; }
  fmt.Println("Success deleting MI: ", mi.GetName())
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
    err := cfg.Delete(mi)
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
