package main

import (
	"fmt"
	"sync"

	//"exec" // Old ? Also: "os/exec" https://stackoverflow.com/questions/6182369/exec-a-shell-command-in-go
	"encoding/json"
	"errors"
	"os"
	"os/exec"

	MIs "github.com/ohollmen/goclowdy/MIs"
	//"golang.org/x/exp/constraints" // go mod download golang.org/x/exp
)
// Test array for "array-of-strings" chunking.
var names2 = []string{"bu1","bu2","bu3","bu4","bu5","bu6","bu7","bu8","bu9","bu10","bu11","bu12","bu13","bu14","bu15","bu16",};

// Chunk an array of strings (mostly for testing)
// TODO: Generics: replace string w. T
// https://go.dev/blog/intro-generics
// Call by chunk[string](items, 10)
// [T constraints.Ordered] [T any]
// https://www.digitalocean.com/community/tutorials/how-to-use-generics-in-go
// # Iter of 718 MIs done
// chunk size (3) exceeds array size (0) !
func chunk(items []string, sasize int) [][]string {
  var chunks [][]string
  alen := len(items)
  debug := true // TODO: Pass
  if alen == 0 { fmt.Printf("array size is 0 - can't extract any chunks !\n"); return nil }
  if sasize > alen { fmt.Printf("chunk size (%d) exceeds array size (%d) !\n", sasize, alen); return nil }
  for i := 0; (i + sasize) < (alen+3); i += 3 {
    j := i + sasize
    if j > (alen) { j = alen; } // Cap j (to not exceed arr. boundaries)
    if debug { fmt.Printf("Getting slice of %d:%d (len:%d)\n", i, j, alen); }
    // Note: range end is non-inclusive index
    chunks = append(chunks, items[i:j])
  }
  if debug { fmt.Printf("Items: %+v\n", chunks); }
  return chunks
}
//i := 3
//j := 50
//sasize := 3
//fmt.Printf("Items: %+v\n", names3);
    //names3[0] = "buX"
//fmt.Printf("Items: %+v\n", names2);

// Chunk an array-of-MIMI items.
// Used in "chunk-strategy" of dividing MIMI items for (deletion) processing.
func chunk_mimi(items []MIMI, sasize int) [][]MIMI {
	var chunks [][]MIMI
	alen := len(items)
	debug := true // TODO: Pass
	if sasize > alen { fmt.Printf("chunk size (%d) exceeds array size (%d) !\n", sasize, alen); return nil }
	for i := 0; (i + sasize) < (alen+3); i += 3 {
	  j := i + sasize
	  if j > (alen) { j = alen; } // Cap j (to not exceed arr. boundaries)
	  if debug { fmt.Printf("Getting slice of %d:%d (len:%d)\n", i, j, alen); }
	  // Note: range end is non-inclusive index
	  chunks = append(chunks, items[i:j])
	}
	if debug { fmt.Printf("Items: %+v\n", chunks); }
	return chunks
  }

// Test for WaitGroup run (individual) task callback: launch a shell command ("os/exec", "exec" seems to be outdated)
func hello(str string, wg *sync.WaitGroup) error {
  fmt.Printf("Hello %s\n", str);
  defer wg.Done()
  cmd := exec.Command("sleep", "2")
  stdout, err := cmd.Output()
  if err != nil { return nil }
  fmt.Printf("Cmd output: %s\n", stdout);
  //exec: cmd.Close()
  return nil
}
//cmd, err := exec.Run("/bin/ls", []string{"/bin/ls"}, []string{}, "", exec.DevNull, exec.PassThrough, exec.PassThrough)
  //if (err != nil) {
  //  fmt.Println(err)
  //  return
  //}
  //cmd := exec.Command("ls")

// Helper callback for chunk-based deletion (called for each item from mimilist_del_chunk() )
func mic_delete_mi_wg(mic * MIs.CC, mimi * MIMI, wg *sync.WaitGroup) error {
  defer wg.Done()
  //fmt.Printf("SHOULD-DELETE: %s (%s)\n", mimi.miname, verdict[mimi.class]); return nil // Debug: Simulate no error
  err := mic.Delete(mimi.miname);
  if err != nil { fmt.Printf("Error: Failed deleting MI '%s' during parallel processing !\n", mimi.miname); return err; }
  return nil;
}
// Load JSON Config (for MI deletion, VM listing ...)
func config_load(fname string, mic * MIs.CC) error {
  if fname == "" { fname = "./goclowdy.conf.json" }
  // Check file presence !!
  stinfo, err := os.Stat(fname)
  if os.IsNotExist(err) { return err }
  if stinfo.IsDir() { return errors.New("Config file is a DIR!") }
  dat, err := os.ReadFile(fname)
  if err != nil { return errors.New("Config Not found ") } // + string(err)
  err = json.Unmarshal(dat, mic)
  if err != nil { return errors.New("Config Parsing failed: ") } // + string(err)
  //if mic.Debug { fmt.Printf("Config: %+v\n", mic); }
  return nil
}

func subarr_test() {
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
}
