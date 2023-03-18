package MIs

import (
  compute "cloud.google.com/go/compute/apiv1"
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  "time"
  "os"
  "fmt" // for name
  "context"
)
// Machine Image Client Config
type CC struct {
  Project string
  CredF string
  TZn string
  WD_keep int
  KeepMinH int // dur_keep_all => KeepMinH
  // Priv
  tnow time.Time
  tloc * time.Location
  //tnow Time
  // ctx !!
  c * compute.MachineImagesClient
}


const (
  KEEP_SAFE int = 0
  KEEP_NEW = 1
  KEEP_WD = 2
  DEL_1W  = 3
  DEL_1Y  = 4
)

//func (cfg * CC) Client() {
//  
//}
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
  return 0
}

func Classify(mi * computepb.MachineImage, cfg * CC) int {
  t, err := time.ParseInLocation(time.RFC3339, mi.GetCreationTimestamp(), cfg.tloc) // Def. UTC
  nd := cfg.tnow.Sub(t) // Duration
  if err != nil { return KEEP_SAFE; }
  hrs := nd.Hours()
  if hrs > (24 * (365 + 7)) { return DEL_1Y; } // fmt.Println("DEL ");
  // Less than MAX period (e.g. 1Y+1W) old ... but
  // Test for always keep-period
  // float64
  if hrs < float64(cfg.KeepMinH) {  return KEEP_NEW; } // fmt.Println("KEEP (new, < week old)");
  if cfg.WD_keep == int(t.Weekday()) {  return KEEP_WD; } // fmt.Println("KEEP correct day");
  
  //fmt.Println(nd)
  return DEL_1W
}

// To delete .. based on class.
func To_be_deleted(cl int) bool {
  if (cl >= DEL_1W) || (cl <= DEL_1Y) { return true }
  return false
}

// 
func (cfg * CC) Delete(mi * computepb.MachineImage, c * compute.MachineImagesClient) {
  fmt.Println("Should delete "+ mi.GetName());
  //dreq := &computepb.DeleteMachineImageRequest{MachineImage: mi.GetName(), Project: project} //RequestId
  //dreq.Reset()
  //op, err := c.Delete(ctx, dreq)
  //if err != nil {}
  //err = op.Wait(ctx)
  //if err != nil {}
}

// inst * computepb.Instance (Instead of mini, make variadic ?)
func (cfg * CC) Create(mini * string, c * compute.MachineImagesClient) { // , cfg * CC
  
  var storageLocation []string
  storageLocation = append(storageLocation, "us")
  //mini := StdName(inst.GetName())
  instance_path := "" // pass: Instance
  req := &computepb.InsertMachineImageRequest{
    Project: cfg.Project,
    SourceInstance: &instance_path,
    MachineImageResource: &computepb.MachineImage{
      Name: mini, //&imageName,
      SourceInstance: &instance_path,
      StorageLocations: storageLocation,
    },
  }
  ctx := context.Background()
  op, err := c.Insert(ctx, req) // cfg.c
  instancename := *mini
  if err != nil {
    fmt.Printf("Error in Insert - vm image failed for instance %s - %v\n", instancename , err)
    //return(err)
  }
  err = op.Wait(ctx)
  if err != nil {
    fmt.Printf("Error in Wait(ctx) - vm image failed for instance %s - %v\n", instancename , err)
    //return(err)
  }
}

func (cfg * CC) GetOne(in string, c * compute.MachineImagesClient) *computepb.MachineImage {
  ctx := context.Background()
  req := &computepb.GetMachineImageRequest{MachineImage: in, Project: cfg.Project}
  // https://pkg.go.dev/cloud.google.com/go/compute/apiv1#MachineImagesClient.Get
  mi, err := c.Get(ctx, req)
  if err != nil { fmt.Println("Error fetching i"); return nil }
  return mi
}
// ...StdName(i.GetName()). TODO: Opt date
func StdName(mn string) string {
  now := time.Now()
  y, m, d := now.Date()
  dtstr := fmt.Sprintf("%d-%.2d-%.2d",y, int(m), d)
  return mn+"-"+dtstr
}

//func instpath(inst *computepb.Instance, project string) string {
//  return fmt.Sprintf( "%s/%s/%s/%s/%s/%s", "projects", project, "zones", instzone(inst), "instances", inst.GetName() )
//}

//func instzone(inst *computepb.Instance) string {
//  fullzone := inst.GetZone()
//  li := strings.LastIndex(fullzone, "/")
//  return fullzone[li+1:len(fullzone)]
//}
