
package VMs

import (
  compute "cloud.google.com/go/compute/apiv1"
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  //"time"
  "os"
  "fmt"
  "context"
  "strconv"
  "google.golang.org/api/iterator"
  "google.golang.org/protobuf/proto"
  "sync"
  "unsafe"
  "strings"
  "time"
  "regexp"
)
// VM Client Config
type CC struct {
  Project string
  //CredF string
  c * compute.InstancesClient
  Debug bool
}

const (CB_NO_USERDATA int =0; CB_W_USERDATA = 1)
type IterCfg struct {
  CBSign int // For now 
  Userdata unsafe.Pointer
  //wg sync.WaitGroup
  TimeDurS int
}
// Wide-use App (CLI,Web,CF) Infra context params for uses like
// - Filtering by Zones, Host-Pattern, Labels
type InfraPara struct {
  Project string `json:"project"`
  Zones []string `json:"zones"`
  //Regions []string `json:"regions"`
  Patt string `json:"patt"`
  Re *regexp.Regexp
  Labels map[string]string `json:labels` // Note: Exact match k-v
  Force bool `json:"force"`
  AltSuff string `json:"altsuff"`
  //Ts int64
  Debug bool `json:"debug"`
}

func (cfg * CC) Init() int {
  ctx := context.Background()
  // && ! cfg.Project
  if os.Getenv("GCP_PROJECT") != "" { cfg.Project = os.Getenv("GCP_PROJECT") }
  cfg.c, _ = compute.NewInstancesRESTClient(ctx)
  //if err != nil { return 1 }
  return 1
}
func (cfg * CC) GetAll() []*computepb.Instance { // ctx context.Context,
  //project := cfg.Project
  //ctx := cfg.ctx
  ctx := context.Background()
  var vm_all [](*computepb.Instance)
  // Test count
  // From outside
  if cfg.c == nil { fmt.Println("No client found for search. Check that .Init() was called and creds. set."); return vm_all }
  //c, _ := compute.NewInstancesRESTClient(ctx)
  //if c == nil { fmt.Println("Req not created"); return vm_all }
  //defer c.Close() // Could be still in use
  req := &computepb.AggregatedListInstancesRequest{ Project: cfg.Project, MaxResults: proto.Uint32(1000), }
  if req == nil { fmt.Println("Req not created"); return vm_all }
  it := cfg.c.AggregatedList(ctx, req)
  if it == nil { fmt.Printf("No List\n"); return vm_all; }
  i := 0
  for {
    resp, err := it.Next()
    i += 1
    if err == iterator.Done { break }
    if err != nil {
      fmt.Println("Iter error\n" + strconv.Itoa(i))
      break
    }
    instances := resp.Value.Instances // pair. ?
    if len(instances) > 0 { //  continue
      for _, instance := range instances {
        vm_all = append(vm_all, instance)
      }
    }
    // if cb { }
  }
  return vm_all
}
// 
func (cfg * CC) ForEachVM(iarr []*computepb.Instance, cb func (*computepb.Instance) error ) {
  if cb == nil { fmt.Println("Missing VM iteration callback"); return; }
  for _, item := range iarr{
    //wg.Add(1)
    fmt.Println("Name: ", item.GetName())
    err := cb(item);
    if err != nil { fmt.Println("Error iterating VM ", item.GetName()); }
  }
}
// Parallel
func (cfg * CC) ForEachVMPar(iarr []*computepb.Instance, cb func (*computepb.Instance, unsafe.Pointer) error, icfg * IterCfg) { // userdata unsafe.Pointer
  if cb == nil { fmt.Println("Missing VM iteration callback"); return; }
  // Create wg
  var wg sync.WaitGroup
  for _, item := range iarr{
    wg.Add(1) // if wg
    fmt.Println("Name: ", item.GetName())
    go cb(item, icfg.Userdata);
    wg.Done() // if wg
  }
  wg.Wait() // if wg
  
}
// VM Filter Function
type VMFF func(* computepb.Instance, InfraPara) bool

// TODO: Instead oc calling as many times as there are filter funcs (leading to caller/call complexity),
// make variadic and pass funcs to use as filter ? Would need to AND/OR (individual filter) results ?
func Filter(inarr []* computepb.Instance, f VMFF, p InfraPara) []* computepb.Instance {
  oarr := make([]* computepb.Instance, 0)
  for _, vm := range inarr {
    if f(vm, p) { oarr = append(oarr, vm) }
  }
  return oarr
}

// Extract project name
func VMProj( vm * computepb.Instance ) string {
  sl := vm.GetSelfLink()
  off := strings.Index(sl, "projects/")
  // fmt.Println("off:", off)
  rem := sl[off+9:len(sl)-1]
  // fmt.Println("rem:", rem)
  off = strings.IndexByte(rem, '/')
  p := rem[0:off]

  //return "", errors.New("empty name")
  return p
}

func ISODate() string {
  now := time.Now()
  y, m, d := now.Date()
  return fmt.Sprintf("%d-%.2d-%.2d",y, int(m), d)
}
