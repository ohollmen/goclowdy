package main
import (
  "fmt"
  "sync"
  //"exec" // Old ? Also: "os/exec" https://stackoverflow.com/questions/6182369/exec-a-shell-command-in-go
  "os/exec"
  MIs "github.com/ohollmen/goclowdy/MIs"
)
var names2 = []string{"bu1","bu2","bu3","bu4","bu5","bu6","bu7","bu8","bu9","bu10","bu11","bu12","bu13","bu14","bu15","bu16",};

// TODO: Generics: replace string w. 
func chunk(items []string, sasize int) [][]string {
  var chunks [][]string
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
//i := 3
//j := 50
//sasize := 3
//fmt.Printf("Items: %+v\n", names3);
    //names3[0] = "buX"
//fmt.Printf("Items: %+v\n", names2);

func hello(str string, wg *sync.WaitGroup) error {
  fmt.Printf("Hello %s\n", str);
  defer wg.Done()
  //cmd, err := exec.Run("/bin/ls", []string{"/bin/ls"}, []string{}, "", exec.DevNull, exec.PassThrough, exec.PassThrough)
  //if (err != nil) {
  //  fmt.Println(err)
  //  return
  //}
  //cmd := exec.Command("ls")
  cmd := exec.Command("sleep", "2")
  stdout, err := cmd.Output()
  if err != nil { return nil }
  fmt.Printf("Cmd output: %s\n", stdout);
  //exec: cmd.Close()
  return nil
}

func delete_wg(mic * MIs.CC, mimi * MIMI, wg *sync.WaitGroup) error {
  defer wg.Done()
  err := mic.Delete(mimi.miname);
  if err != nil { fmt.Printf("Error: Failed deleting MI '%s' during parallel processing !\n", mimi.miname); return err; }
  return nil;
}
