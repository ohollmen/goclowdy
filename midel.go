// Machime image (MI) Deletion functionality.
// Move here: mi_del, mimilist_del* (3x), mic_delete_mi, miclstat_out
package main
import (
  "context"
  "fmt"
  "sync"
  computepb "cloud.google.com/go/compute/apiv1/computepb"
  "google.golang.org/api/iterator"
  "time"
  "math/rand"
  //"os" // os.Exit(rc)
  MIs "github.com/ohollmen/goclowdy/MIs"
  "github.com/ohollmen/goclowdy/workerpool"
)
//var dummy int = 5;

// Machine image mini-info. Allow deletion to utilize this (for reporting output). Add creation time ?
type MIMI struct {
  miname string
  class int
  ctime time.Time
}

// Delete machine image/MIMI (Used for serial and channels (workerpool) based deletion).
func mic_delete_mi(mic * MIs.CC, mimi * MIMI) int { // mi *computepb.MachineImage
  err := mic.Delete(mimi.miname) // mi.GetName()
  if err != nil { fmt.Printf("Error Deleting MI: %s\n", mimi.miname); return 1
  } else {
   fmt.Printf("Deleted %s\n", mimi.miname); return 0
  }
  //fmt.Printf("Should have deleted %s. Set DelOK (MI_DELETE_EXEC) to actually delete.\n", mi.GetName())
  //return 0
}

// Delete MIs beyond max age.
// This is custom case for max age, partially to proto/prove/test the mic.GetAll() approach.
func mi_del_max() {
  // Currently Project gets set in mic
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic); return; }
  agelimit := float64(mic.KeepMaxH) // Once, outside loop.
  // Calc cut-off date to display to user
  // https://stackoverflow.com/questions/37697285/how-to-get-yesterdays-date-in-golang
  told := mic.Tnow().AddDate(0, 0, -(mic.KeepMaxH / 24))
  //told := mic.Tnow().Add( int64(mic.KeepMaxH) * time.Hour ) // - invalid operation: mic.KeepMaxH * time.Hour (mismatched types int and time.Duration) // or int64 and ...
  fmt.Printf("Max-Old boundary date: %s\n", (&told).Format("2006-01-02") )
  //if true { return } 
  all := mic.GetAll()
  if all == nil { fmt.Printf("No machine images gotten.\n"); return }
  fmt.Printf("Got initial set of %d MIs (in %s)\n", len(all), mic.Project); // return
  todel := []*computepb.MachineImage{}
  if todel == nil { return; }
  for _, mi := range all {
    tc, err := mic.CtimeUTC(mi)
	if err != nil {}
	// Older than nax
    age := mic.AgeHours2(tc)
    if age >= agelimit {
      fmt.Printf("DELETE %s (%s)\n", mi.GetName(), (&tc).Format("2006-01-02")) // "2006-01-02T15:04:05-0700"
      todel = append(todel, mi)
    }
  }
  if len(todel) < 1 { fmt.Printf("No MIs (older than max axe) to delete"); return }
  fmt.Printf("%d MIs to delete (out of %d)\n", len(todel), len(all));
  /// Delete ...
}

// Delete machine images per given config / policy (sourced from cfg or global defaults).
// ONLY needs access to MIs (Uses mic for that).
// TODO:
// - Possibly Convert to use getAll, except we want MIMI (not full computepb.MachineImage ents)
// - 3 pass: 1) Get items, 2) classify 3) delete
// https://code.googlesource.com/gocloud/+/refs/tags/v0.101.1/compute/apiv1/machine_images_client.go
// Old raw: //var delarr []*computepb.MachineImage // var item *computepb.MachineImage
func mi_del() { // pname string
  ctx := context.Background()
  //config_load("", &mic); // Already on top
  // flag.Parse() // TODO. Ok ? Not done before ?
  rc := mic.Init()
  if rc != 0 {fmt.Printf("MI Client Init() failed: %d (%+v)\n", rc, &mic);  }
  if Delok { mic.DelOK = true }
  fmt.Printf("Config (after init): %+v\n", &mic);
  if rc != 0 { fmt.Printf("Machine image module init failed: %d\n", rc); return }
  // Classification stats. Note: no wrapping make() needed w. element initialization
  miclstat := map[int]int{0: 0, 1:0, 2:0, 3:0, 4:0, 5:0}
  //TEST: miclstat_out(mic, miclstat); return;
  // TODO: if Project != "" { mic.Project = Project; }
  fmt.Println("Search MIs from: "+mic.Project+", parse by: "+time.RFC3339)
  totcnt := 0; // todel := 0; // TODO: elim todel (get from slice)
  var delarr []MIMI
  verbose := true
  // all := mic.GetAll()
  //if all == nil { fmt.Printf("No machine images gotten."); return }
  // totcnt := len(all)
  if mic.Project == "" { fmt.Println("No Project scope available for deletion (from config, cli or env)"); return }
  var maxr uint32 = 500 // 20
  req := &computepb.ListMachineImagesRequest{ Project: mic.Project, MaxResults: &maxr } // Filter: &mifilter } // 
  it := mic.Client().List(ctx, req)
  if it == nil { fmt.Println("No MIs from Project: "+mic.Project); return; }
  // Iterate MIs, check for need to del
  
  for {
    //fmt.Println("Next ...");
    mi, err := it.Next()
    if err == iterator.Done { fmt.Printf("# Iter of %d MIs done\n", totcnt); break } // At the end of normal iter. w. results
    if mi == nil {  fmt.Println("No mi. check (actual) creds, project etc."); break } // No results (e.g. wrong creds)
    /////// Actual processing ////////
    // NEW: Use discovered UTC C-TS (From client, shared). TODO: Add to mimi (in delarr) ?
    t, err := mic.CtimeUTC(mi) // TODO: (later ?) to mimi.ctime
    if err != nil { fmt.Println("Create-time not parsed !");break; }
    // OLD: mi.GetCreationTimestamp()
    if verbose { fmt.Println("MI:"+mi.GetName()+" (MICreated: "+t.Format(time.RFC3339)+" "+wdnames[t.Weekday()]+")") } // Now w. weekday
    var cl int = mic.Classify(mi)
    if verbose { fmt.Println(verdict[cl]) }
    miclstat[cl]++
    if MIs.ToBeDeleted(cl) {
      //todel++
      if verbose { fmt.Printf("DELETE %s\n", mi.GetName()) } // Also in DRYRUN
      mimi := MIMI{miname: mi.GetName(), class: cl, ctime: t} // Add time, WD ?
      // mimi.ctime = t
      delarr = append(delarr, mimi) //  OLD: store full mi
    } else {
      if verbose { fmt.Printf("KEEP %s\n", mi.GetName()) }
    }
    if verbose { fmt.Printf("============\n") } // divider
    totcnt++
  }
  // Dry-run (or no ents) - terminate here
  if !mic.DelOK       { fmt.Printf("# Dry-run mode, (DelOK = %t, %d to delete)\n", mic.DelOK, len(delarr)); miclstat_out(mic, miclstat);return; }
  if len(delarr) == 0 { fmt.Printf("# Nothing to Delete (DelOK = %t, %d to delete)\n", mic.DelOK, len(delarr)); miclstat_out(mic, miclstat);return; }
  // Delete items from delarr - either serially or in chunks or using channels
  // func mimilist_delete(mic MIs.CC, delarr []MIMI) {
  if mic.ChunkDelSize == -1 {
    mimilist_del_chan(mic, delarr)
  } else if mic.ChunkDelSize == 0 {
    mimilist_del_serial(mic, delarr)
  } else {
    mimilist_del_chunk(mic, delarr)
  }
  //}
}

// MI Classification stats output (mainly for targeting deletion) 
func miclstat_out(mic MIs.CC, miclstat map[int]int) {
  // Need: .UTC(). ?
  fmt.Printf("MI Class (keep/delete reasoning) stats (%s, %s):\n", mic.Project, time.Now().Format("2006-01-02T15:04:05-0700")); // \n%+v\n", miclstat (raw dump)
  clsarr := []int{MIs.KEEP_SAFE, MIs.KEEP_NEW, MIs.KEEP_WD, MIs.KEEP_MD, MIs.KEEP_CUSTOM,    MIs.DEL_1W, MIs.DEL_OLD}
  //for key, value := range miclstat { // Old map-iter (keys in random order)
  for i := 0; i < len(clsarr); i++ {
    key := clsarr[i]
    fmt.Println("Class:", verdict[key], "(",key,") :", miclstat[key]) // OLD: value
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
	fmt.Printf("Channel based processing completed !\n"); // of %d items, len(delarr)
  }


