# Tips and tricks on Cloud functions

## Call handling - HTTP vs. PubSub

Cloud functions are typically trigered by:
- HTTP Calls
- Pub-Sub messages
The function signatures for these call types are different ( `...(w http.ResponseWriter, r *http.Request)` for HTTP vs.
`...(ctx context.Context,m PubSubMessage) error` for CGP PubSub).

Despite this difference it is possible to make Cloud function source to implement 2 entrypoints, one for each call interface
and keep the source code unified between the 2. However Google Cloud Console GUI does not have the flexibility to share single
code deployment and have 2 named function declarations.

Also one of the call interfacing functions can be made to "redirect" to the the other call interface function (for maximum code sharing and minimum code duplication).
An example here illustrates redirecting HTTP call handler to PubSub handler (Handler basename here for CF task is `MyTask`):

```
// Sidenote: Cannot be 'main' (for CF)
package mytask
import (
  ...
  "fmt"
  "net/http"
  "encoding/json"
  ...
)
// See https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage for what is coming in.
// For this example all data is encoded as JSON in member "data".
type PubSubMessage struct {
  Data []byte `json:"data"`
}
// Wrapper for HTTP Calls
func MyTaskHTTP(w http.ResponseWriter, r *http.Request) {
  ctx := context.Background()
  // Create a stub request object for PubSub "redirection"
  var m PubSubMessage
  //fmt.Fprintf(w, "PubSubMessage %v\n", m)
  var err error
  // Read HTTP Body (to convert to PubSub)
  m.Data, err = ioutil.ReadAll(r.Body)
  //fmt.Fprintf(w, "BODY %v (err=%d)!", m.Data, err) // Note: This goes to HTTP resp
  if (err != nil) { w.WriteHeader(400); w.Write([]byte("NOT-OK")); return; }
  //res.status(200).send("OK"); resp.Body.Close()
  // Getting var res *http.Response ? res.ContentLength = len(rdata)
  w.WriteHeader(200)
  // TODO: Increase sophistication here ("encoding/json") by discovering the relationship between http.ResponseWriter vs. http.Response
  var rdata []byte = []byte ("{\"status\": \"Processing spawned in the Background. See Logs for final results.\"}")
  w.Header().Set("Content-Length", strconv.Itoa(len(rdata)) ) // len(rdata) buf.Len()
  w.Header().Set("Content-Type", "application/json; charset=utf-8")
  w.Write(rdata) // []byte("OK"). TODO: Flush/Close output (do not let buffering happen)
  // io.WriteString(w, "STR\n")
  MyTask(ctx,m) // Call PubSub handler (m = PubSubMessage)
}

// PubSub handler. Write responses to logs, send email, or write to DB here.
func MyTask(ctx context.Context, m PubSubMessage) error {
  err := json.Unmarshal(m.Data, &mytype)
  // The bulk of processing starts here
  ...
}

```

## Access to "FS-like" data

Despite the statelessness of Cloud function and most function deployments doing all processing "in memory" (wihout need to access static files, configs, etc.),
it seems that GCP facilitates the original source code directory as minimal read-only "filesystem area" to cloud function (available as subdirectory `./serverless_function_source_code`).
See Document "Cloud Functions execution environment" => Section "Memory and file system" (URL in Refs) for more info.

## Package for cloud function

- Cannot be main
- Since (files in) one dir can handle only one (package) namespace, the possible CLI utils (or tests ?) budled onto same project must be in different (sub)dir
- Either:
  - Place CF to topdir, other(s), e.g. CLI util to subdir(s)
  - Place CLI util to topdir, CF to subdir ... for no namespace clashes

## Data in PubSub Message

PubSub messages have (JSON) members as documented in: https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
In the GCP Cloud console there are fields for "Message Body (=> "data", type string) and "Message Attributes" ("attributes", type object of keys and string values).
There are also members PubsubMessage (string), PubsubMessage (string, RFC3339), orderingKey (string, related messages ...), see doc for details on those.
App can take a stance to deliver all params in member "data" as for example JSON text string (e.g. `{ "action": "create", "name": "...." }`
this is what would be shown in "Message Bosy") and ignore all attributes as parameter delivery mechanism.
On the other hand all params could be passed in attributes (soem apps do this).

In PubsubMessage "data" is documented as base64 encoded string (and in TF ...), but in none of the PubSub cloud function examples there is any base64 decoding happening,
so this seems to be handled before message is passed to handler.

See "Pub/Sub client libraries" (Note: in go lib cloud.google.com/go/pubsub, the passed PubSub message seems to be of type `m *Message`, but has member m.Data, string).
The documentation also shows how to create a new PubSub client (e.g. for testing):
```
import (
  "context"
  // See: https://pkg.go.dev/cloud.google.com/go/pubsub
  "cloud.google.com/go/pubsub"
)
ctx := context.Background()
// ctx context.Context, projectID string, config *ClientConfig, option.ClientOption, (c *Client, err error)
client, err := pubsub.NewClient(ctx, projID, opts...)
// ... OR with ClientConfig
client, err := pubsub.NewClientWithConfig(ctx, projID, config, opts...)
// Note: type Message = ipubsub.Message, where: ipubsub "cloud.google.com/go/internal/pubsub" (https://github.com/googleapis/google-cloud-go/blob/pubsub/v1.29.0/pubsub/message.go#L44)
// Data, Attributes, ID, PublishTime,DeliveryAttempt, OrderingKey (ack has: receiveTime,doneFunc,ackResult)
res := topic.Publish(ctx, &pubsub.Message{Data: []byte("Hello!")})
```

## References

- Write Cloud Functions (Types of Cloud Functions) - https://cloud.google.com/functions/docs/writing#structuring_source_code
- Cloud Functions execution environment - https://stackoverflow.com/questions/28891531/piping-http-response-to-http-responsewriter
- PubsubMessage - https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
- Pub/Sub client libraries - https://cloud.google.com/pubsub/docs/reference/libraries
- Cloud PubSub - Package cloud.google.com/go/pubsub (v1.29.0) - https://cloud.google.com/go/docs/reference/cloud.google.com/go/pubsub/latest
