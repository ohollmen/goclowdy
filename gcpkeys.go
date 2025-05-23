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
	"strings"

	b64 "encoding/base64"

	"google.golang.org/api/iam/v1"
  "time" // time delta / age (key)
  "flag"
)

type KeyInfo struct {
  TokenURI string `json:"token_uri"` // Make this into a complete JWT struct (inc. even the mems that we do not care so much about)
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

var KeyMaxAgeHrs int = 2160;
// N/A: See keyinfo_load
//func get_key_context(acct_p *string, pname_p *string) {}

// List all keys for a user.
// https://cloud.google.com/iam/docs/keys-list-get#go
// https://pkg.go.dev/google.golang.org/api/iam/v1#ServiceAccountKey
// https://pkg.go.dev/google.golang.org/api/iam/v1 ProjectsServiceAccountsKeys "google.golang.org/api/iam/v1".DisableServiceAccountKeyRequest
// See also (ALT) IamClient: https://pkg.go.dev/cloud.google.com/go/iam/admin/apiv1
// Similar to: gcloud iam service-accounts keys list --iam-account ... --project ...
func key_list() {
  flag.Parse()
  allowdisa := false
  if Delok { allowdisa = true; }
	ctx := context.Background()
	iamService, err := iam.NewService(ctx) // option.WithAPIKey("AIza...")
	if err != nil { fmt.Println("No IAM Service: ", err);return}
	//ki := &KeyInfo{}
	//if ki == nil { return }
	//var pname string
	//ki.Project = mic.Project
  var ki *KeyInfo = nil;
  
	akfn := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
  sapath := os.Getenv("SA_PATH"); // export SA_PATH=projects/PROJ/serviceAccounts/KEYID
  sapath_src := "ENV"
  // SA (--sa) From Commandline
  if SAEmail != "" {
    if !strings.Contains(SAEmail, "@") { fmt.Printf("Service account should be given as email address !\n"); return; }
    acc_serv := strings.Split(SAEmail, "@")
    if len(acc_serv) != 2 {  fmt.Printf("Service account should be given as email address - is in wrong format !\n"); return; }
    proj_junk := strings.Split(acc_serv[1], ".")
    if proj_junk[0] == "" { fmt.Printf("Service account does not seem to contain project name !\n"); return; }
    sapath = fmt.Sprintf("projects/%s/serviceAccounts/%s", proj_junk[0], SAEmail);
    fmt.Printf("Formed SAPath '%s' based on CLI\n", sapath);
    sapath_src = "CLI"
  // Env. SA_PATH
  } else if sapath != "" {
    if !strings.Contains(sapath, "@") { fmt.Printf("Service account should be given as email address - is in wrong format !\n"); return; }
    if !strings.Contains(sapath, "/") { fmt.Printf("Service account should be given as email address - is in wrong format !\n"); return; }
  } else if akfn != "" { //if akfn == "" {fmt.Printf("SA Key file was not passed in env (GOOGLE_APPLICATION_CREDENTIALS)\n"); return}
    ki = keyinfo_load(akfn)
    if ki == nil { fmt.Printf("SA Key (%s) not loaded !\n", akfn); return }
    //fmt.Printf("Key loaded from '%s': %v\n", akfn, ki);
    sapath = fmt.Sprintf("projects/%s/serviceAccounts/%s", ki.Project, ki.Email);
    sapath_src = "KEY_FILE"
    //fmt.Printf("Loaded keypath from %s): '%s'\n", sapath_src, sapath);
  }
  if sapath == "" { fmt.Printf("No SA Path available (for key list search) from ENV (SA_PATH=/projects/${proj}/serviceAccounts/${email}) or JSON keyfile (by GOOGLE_APPLICATION_CREDENTIALS).\n");return }
  fmt.Printf("Using SA keypath from %s: '%s'\n", sapath_src, sapath);
	// Expired filter: "validBeforeTime<0d"
	resp, err := iamService.Projects.ServiceAccounts.Keys.List(sapath).Context(ctx).Do()
	if err != nil {fmt.Printf("No Keys Found: %v\n", err);return}
	//fmt.Println("Got:", resp) // iam.ListServiceAccountKeysResponse
	//fmt.Printf("%T\n", resp) // import "reflect" fmt.Println(reflect.TypeOf(tst)) // DEBUG
	for _, key := range resp.Keys {
		// TODO: Time dur here
		//fmt.Printf("%T\n", key) // *iam.ServiceAccountKey
    //tm := time.Now()
    //key.ValidAfterTime = key.ValidAfterTime[0:10]
    key.ValidAfterTime = strings.Replace(key.ValidAfterTime, "T", " ", 1); // Max 1
    // NOTE: golang fails to parse ISO time with T between date and time !!!
    tc, err := time.Parse("2006-01-02 15:04:05Z", key.ValidAfterTime) // T15:04:05
    if err != nil { fmt.Printf("Failed to parse %s\n", key.ValidAfterTime); continue; }
    duration := time.Now().Sub(tc) // Age days. Sub() needs time.Time
    //agedays := fmt.Sprintf("%d", int(duration.Hours()/24))
    agehrs := int(duration.Hours())
    //agedays := int(duration.Hours()/24)
    agedays := duration.Hours()/24 // Float
    //agedays := duration.String()
    //fmt.Println(tc)
    exceed := int(agehrs - KeyMaxAgeHrs);
    // ExtendedStatus
    //fmt.Printf("KP(full): %s\n", key.Name);
		fmt.Printf("%v Crea/Exp.: %s..%s (%.1f d.), Disa: %v, Orig: %s, MngdBy:%s\n",
       path.Base(key.Name), key.ValidAfterTime, key.ValidBeforeTime, agedays, key.Disabled, key.KeyOrigin, key.KeyType)
    if (agehrs > KeyMaxAgeHrs) && !key.Disabled && (key.KeyType != "SYSTEM_MANAGED") {
      fmt.Printf("- Should disable the Key '%s' (Age: %.1f d., exceed by %d hrs)\n", key.Name, agedays, exceed);
      if !allowdisa { fmt.Printf("- ... However disablement is not allowed (use --delok to disable)\n"); continue; }
      //sapath_k := fmt.Sprintf("%s/keys/%s", sapath, ); // HARD Way
      // https://pkg.go.dev/google.golang.org/api/iam/v1#DisableServiceAccountKeyRequest
      // https://cloud.google.com/iam/docs/service-accounts-disable-enable#go (has example)
      //var disa DisableServiceAccountKeyRequest = DisableServiceAccountKeyRequest{ServiceAccountKeyDisableReason: "Key has expired"}
      
      disareq := iam.DisableServiceAccountKeyRequest{} // ServiceAccountKeyDisableReason: "Key has reached max allowed age"
      //disareq.??? = "Key has reached max allowed age" // ServiceAccountKeyDisableReason
      resp, err := iamService.Projects.ServiceAccounts.Keys.Disable(key.Name, &disareq).Context(ctx).Do()
      if err != nil { fmt.Printf("Failed to disable key: '%s' %s\n", key.Name, err); }
      if resp != nil {}
      
    }
	}
	// Also: SignJwtRequest, but: https://cloud.google.com/iam/docs/migrating-to-credentials-api
	//auth_token()
}
// Get access/auth token from gcloud auth print-access-token.
// Make sure correct (sufficiently privileged) account is active (gcloud config get account --quiet)
// Detect exit values !=0 => Output is not an auth token. See need for --quiet.
// Return empty string on errors / available, valid (ready-to-use) key-string otherwise.
// See also: https://stackoverflow.com/questions/72275338/get-access-token-for-a-google-cloud-service-account-in-golang
func auth_token() string {
  cmd := exec.Command("gcloud", "auth", "print-access-token") // "--quiet"
  stdout, err := cmd.Output() // stdout (openssl does not output anything, only errors matter)
  if err != nil { fmt.Println("gcloud error: ", err.Error()); return "" }
  at := string(stdout);
  at = strings.TrimSpace(at)
  //fmt.Printf("Token: '%s'\n", at);
  return at
}
//acct :=
	//ki.Email = os.Getenv("GCP_SA")
	//pname =
	//ki.Project = os.Getenv("GCP_SA_PROJECT") // override
	//if acct == "" { fmt.Println("No Service account GCP_SA");return}
	//if pname == "" {fmt.Println("No GCP_SA_PROJECT"); return}

// Load JWT Key info from a file given in akfn (auth key filename).
func keyinfo_load(akfn string) *KeyInfo {
  errbase := "keyinfo_load: SA Key not (fully) loaded."
  if akfn == "" { fmt.Printf("%s. Filename not passed (for loading the key)\n", errbase); return nil }
  _, err := os.Stat(akfn)
  if err != nil {fmt.Printf("SA Key file '%s' could not be found: %v\n", akfn, err); return nil}
  dat, err := os.ReadFile(akfn)
  if err != nil {fmt.Printf("SA Key config data could not be loaded (%s): %v\n", akfn, err); return nil}
  ki := &KeyInfo{}
  err = json.Unmarshal(dat, ki)
  if err != nil { fmt.Printf("SA Key config (for %s) could not be parsed: %v\n", akfn, err); return nil}
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
	// TODO: Make a better time-diff analysis of expiry, e.g. expired ... ago, will expire in ... hours / days.
	//var t time.Time
	fmt.Printf("Key ('%s') Expires: %s\n", ki.PkeyId, resp.ValidBeforeTime) // ki.ExpTime
	return
}
// Generate SA Key on the Google side (Pull model)
// 
// For Google created key see: https://cloud.google.com/iam/docs/keys-create-delete (See example func createKey ... &iam.CreateServiceAccountKeyRequest{})
func key_gen_google() {
  flag.Parse()
  ctx := context.Background()
	service, err := iam.NewService(ctx)
	if err != nil {  fmt.Errorf("iam.NewService: %w", err); return; } // return nil,
  email := SAEmail // CLI
  if email == "" { fmt.Printf("No SA email passed from CLI (Use --sa ..)\n"); return; }
	sapath := "projects/-/serviceAccounts/" + email
  request := &iam.CreateServiceAccountKeyRequest{}
	key, err := service.Projects.ServiceAccounts.Keys.Create(sapath, request).Do()
	if err != nil { fmt.Errorf("Projects.ServiceAccounts.Keys.Create: %w", err); return; }
	// key.PrivateKeyData field contains the base64-encoded service account key
	// in JSON format.
	// TODO(Developer): Save the below key (jsonKeyFile) to a secure location - You cannot download it later.
	jkdata, err := b64.StdEncoding.DecodeString(key.PrivateKeyData) // Creates []byte, err
  if err != nil { fmt.Printf("Failed to Decode B64 encoded private key (for writing locally)\n"); return; }
  //EXAMPLE: d1 := []byte("hello\ngo\n")
  kfn := "/tmp/key.json"
  err = os.WriteFile(kfn, jkdata, 0644)
  if err != nil { fmt.Printf("Failed write key locally\n"); return; }
	fmt.Printf("Key %s created successfully (wrote key to '%s')\n", key.Name, kfn)
  return;
}
// generate an RSA key with openssl "DIY" method (followed by upload):
// 
// See also:
// - https://cloud.google.com/iam/docs/keys-create-delete#go
// - https://cloud.google.com/iam/docs/keys-upload#iam-service-accounts-upload-rest

// Seems gcloud beta iam service-accounts keys upload public_key.pem does not have Go API equivalent (only HTTP POST)
// NOTE: The generated openssl keypair is not initially tied to any identity. The privkey stays locall, pubkey is uploaded.
// Because of the raw http API is in use, for now you have to issue command: `gcloud auth print-access-token` to acquire HTTP Authorization Bearer token.
// Note: This solution has slight downside (compared to gcloud) in requiring to launch gcloud auth print-access-token manually (set in env).
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
  urlpath := fmt.Sprintf("/v1/projects/%s/serviceAccounts/%s/keys:upload", ki.Project, ki.Email) // ki.PkeyId in resp.
  if urlpath == "" { return }
  pubkmsg := &PubKey{}
  //pubkmsg.Data = string(k_pub)
  pubkmsg.Data = b64.StdEncoding.EncodeToString(k_pub) // []byte(data)
  out, err := json.MarshalIndent(pubkmsg, "", "  ")
  if err != nil { fmt.Println("Error Serializing PubKey message\n"); return }
  //bearer := "Authorization: Bearer "+ ... // From gcloud auth print-access-token
  // https://pkg.go.dev/net/http
  fmt.Printf("Send Pub to: '%s':\n%s\n", urlpath, string(out) )
  ior := bytes.NewReader(out)
  bt := auth_token()
  if os.Getenv("GCP_BT") != "" { bt = os.Getenv("GCP_BT") }// Bearer token
  if bt == "" { fmt.Printf("No Bearer token gotten from gcloud OR set by GCP_BT (acquire w. gcloud auth print-access-token)"); return; }
  c := &http.Client{}
  //var DefaultClient = &Client{}
  req, err := http.NewRequest("POST", gserv + urlpath, ior); // (*Request, error)
  req.Header.Add("Content-Type", "application/json; charset=utf-8")
  req.Header.Add("Authorization", "Bearer "+bt)
  fmt.Println("Request:", req);
  resp, err := c.Do(req)
  // Below is ONLY fit for simple requests with no heqaders
  //resp, err := http.Post(gserv + urlpath, "application/json", ior) //   // &out is *[]byte

  if err != nil { fmt.Printf("Error submitting public key: %s\n", err); return; }
  if resp.StatusCode != http.StatusOK { fmt.Printf("Bad StatusCode: %d\n", resp.StatusCode); return } // E.g. 400 / Bad Request or 401 / Unauthorized
  // ContentLength seems to always be -1
  //if resp.ContentLength < 2 {fmt.Printf("No sufficient content from key POST (Got %db, Status: %d): %v\n", resp.ContentLength, resp.StatusCode, resp); return;  }
  if resp.Uncompressed != true { fmt.Printf("Error: Discovered compressed body !\n"); return; }
  defer resp.Body.Close()
  // NOTE: The response here has the very same kind of members as the list response
  body, err := io.ReadAll(resp.Body)
  if err != nil {  fmt.Printf("Error reading resp body (with id)\n");return;  }
  // Parse Response content to kur
  kur := &KeyUpResp{}
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
