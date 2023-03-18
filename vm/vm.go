
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
)
// VM Client Config
type CC struct {
  Project string
  //CredF string
  c * compute.InstancesClient
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
  if cfg.c == nil { fmt.Println("No client found for searc"); return vm_all }
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
