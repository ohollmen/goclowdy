package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"google.golang.org/api/iam/v1"
)

type KeyInfo struct {
  TokenURI string `json="token_uri"` // Make this into a complete JWT struct (inc. even the mems that we do not care so much about)
  Type    string  `json:"type"` // service_account
  PkeyId  string `json:"private_key_id"`
  PkeyPEM string `json:"private_key"`
  Project string `json:"project_id"`
  Email   string `json:"client_email"`
  // From HTTP (see different naming conv)
  //ExpTime string `json:"ValidBeforeTime"`
}
type PubKey struct {
  Data string `json:"publicKeyData"`
}
// For all fields see: https://cloud.google.com/iam/docs/keys-upload#iam-service-accounts-upload-rest
type KeyUpResp struct {
  Name string `json:"name"`// Must parse (e.g. ) "projects/my-project/serviceAccounts/my-service-account@my-project.iam.gserviceaccount.com/keys/c7b74879da78e4cdcbe7e1bf5e129375c0bfa8d0"
  ValidBefore string `json:"validBeforeTime"`
  //keyAlgorithm
}
type KeyPolicy struct {
  Exphrs int
  Minleft int
}
func get_key_context(acct_p *string, pname_p *string) {

}
// https://cloud.google.com/iam/docs/keys-list-get#go
func key_list() {
	ctx := context.Background()
	iamService, err := iam.NewService(ctx)
	if err != nil { fmt.Println("No Service: ", err);return}
	var pname string = mic.Project
	acct := os.Getenv("GCP_SA")
	pname = os.Getenv("GCP_SA_PROJECT") // override
	if acct == "" { fmt.Println("No Service account GCP_SA");return}
	if pname == "" {fmt.Println("No GCP_SA_PROJECT"); return}
	sapath := fmt.Sprintf("projects/%s/serviceAccounts/%s", pname, acct)
	resp, err := iamService.Projects.ServiceAccounts.Keys.List(sapath).Context(ctx).Do()
	if err != nil {fmt.Printf("No Keys Found: %v\n", err);return}
	//fmt.Println("Got:", resp) // iam.ListServiceAccountKeysResponse
	fmt.Printf("%T\n", resp) // import "reflect" fmt.Println(reflect.TypeOf(tst))
	for _, key := range resp.Keys {
		fmt.Printf("%T\n", key) // iam.ServiceAccountKey
		fmt.Printf("%v Exp.: %s\n", path.Base(key.Name), key.ValidBeforeTime)
	}
	// Also: SignJwtRequest, but: https://cloud.google.com/iam/docs/migrating-to-credentials-api
}
func keyinfo_load(akfn string) *KeyInfo {
  _, err := os.Stat(akfn)
  if err != nil {fmt.Printf("SA Key file '%s' could not be found: %v\n", akfn, err); return nil}
  dat, err := os.ReadFile(akfn)
  if err != nil {fmt.Printf("SA Key config data could not be loaded (%s): %v\n", akfn, err); return nil}
  ki := &KeyInfo{}
  err = json.Unmarshal(dat, ki)
  if err != nil { fmt.Printf("SA Key config could not be parsed: %v\n", err); return nil}
  return ki
}
// Check and verify the current key
// https://cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts.keys/get
func key_check() {
	akfn := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if akfn == "" {fmt.Printf("SA Key file was not passed in env (GOOGLE_APPLICATION_CREDENTIALS)\n"); return}
	ki := keyinfo_load(akfn) // Load ! fmt.Printf("loaded key\n");
	fmt.Printf("Current key (id): %s\n", ki.PkeyId)
	kpath := fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/%s", ki.Project, ki.Email, ki.PkeyId)
	fmt.Printf("Lookup key: %s\n", kpath)
	ctx := context.Background()
	iamService, err := iam.NewService(ctx)
	resp, err := iamService.Projects.ServiceAccounts.Keys.Get(kpath).Context(ctx).Do()
	if err != nil {fmt.Printf("No key retrieved: %v\n", err); return}
	if resp == nil {fmt.Printf("No response: %v\n", err); return}
	//fmt.Printf("%#v\n", resp)
	//fmt.Printf("%v\n", ki);
	//err = json.Unmarshal(dat, ki)
	//ki.ExpTime = resp.ValidBeforeTime;
	fmt.Printf("Key ('%s') Expires: %s\n", ki.PkeyId, resp.ValidBeforeTime) // ki.ExpTime
	return
}

// generate an RSA key with openssl "DIY" method (followed by upload):
// 
// See also:
// - https://cloud.google.com/iam/docs/keys-create-delete#go
// - https://cloud.google.com/iam/docs/keys-upload#iam-service-accounts-upload-rest
// Seems gcloud beta iam service-accounts keys upload public_key.pem does not have Go API equivalent (only HTTP POST)
// NOTE: The generated openssl keypair is not initially tied to any identity. The privkey stays locall, pubkey is uploaded.
var openssl_tmpl = []string{"req", "-x509", "-nodes", "-newkey", "rsa:4096", "-days", "10", "-keyout", "/tmp/private_key.pem", "-out", "/tmp/public_key.pem", "-subj", "/CN=none"} // openssl ... $exp_d
func key_gen() {
  // Use old key as "template" ?
  akfn := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
  if akfn == "" {fmt.Printf("SA Key file was not passed in env (GOOGLE_APPLICATION_CREDENTIALS)\n"); return}
  ki := keyinfo_load(akfn)
  //fmt.Println(ki)
  //ki := KeyInfo{}
  // https://stackoverflow.com/questions/6182369/exec-a-shell-command-in-go
  //fmt.Sprintf(openssl_tmpl);
  cmd := exec.Command("openssl", openssl_tmpl...)
  _, err := cmd.Output() // stdout (openssl does not output anything, only errors matter)
  if err != nil { fmt.Println("openssl error: ", err.Error()); return }
  // Print the output
  fmt.Println("Generated SSH key(s)") // string(stdout)
  k_priv, err := os.ReadFile("/tmp/private_key.pem")
  if err != nil { fmt.Printf("Failed to create private key: %s\n", err);return; }
  k_pub, err := os.ReadFile("/tmp/public_key.pem")
  if err != nil { fmt.Printf("Failed to create public key: %s\n", err);return; }
  fmt.Printf("priv:%s\npub: %s\n", k_priv, k_pub);
  // email should end w. .iam.gserviceaccount.com
  gserv := "https://iam.googleapis.com"
  urlpath := fmt.Sprintf("%s/v1/projects/%s/serviceAccounts/%s/keys:upload", gserv, ki.Project, ki.Email) // ki.PkeyId in resp.
  if urlpath == "" { return }
  pubkmsg := &PubKey{}
  pubkmsg.Data = string(k_pub);
  out, err := json.MarshalIndent(pubkmsg, "", "  ")
  if err != nil { fmt.Println("Error Serializing PubKey message\n"); return }
  //bearer := "Authorization: Bearer "+ ... // equiv of gcloud auth print-access-token
  // https://pkg.go.dev/net/http
  fmt.Println("Send Pub to: %s:\n%s\n", urlpath, string(out) )
  ior := bytes.NewReader(out)
  resp, err := http.Post(gserv + urlpath, "application/json", ior) //   // &out is *[]byte
  if err != nil { fmt.Printf("Error submitting public key: %s\n", err); return; }
  if resp.ContentLength < 2 {fmt.Printf("No sufficient content from key POST (Got %db): %s\n", resp.ContentLength, err); return;  }
  if resp.StatusCode != http.StatusOK { fmt.Printf("StatusCode %d\n", resp.StatusCode); return }
  defer resp.Body.Close()
  body, err := io.ReadAll(resp.Body)
  if err != nil {  fmt.Printf("Error reading resp body (with id)\n");return;  }
  // Parse Response content to kur
  kur := KeyUpResp{}
  err = json.Unmarshal(body, kur)
  if err != nil { fmt.Println("Failed to parse (PubKey upload) response: %s\n", err);return;  }
  id := filepath.Base(kur.Name)
  kinew := KeyInfo{"https://oauth2.googleapis.com/token", "service_account", id, string(k_priv), ki.Project, ki.Email} // 
  
  out, err = json.MarshalIndent(kinew, "", "  ")
  if err != nil { fmt.Printf("Error serializing JWT: %s\n", err); return; }
  fmt.Printf("Success creating key:\n%s\n", out);
  fmt.Printf("Expires: %s\n", kur.ValidBefore)
  return
}
