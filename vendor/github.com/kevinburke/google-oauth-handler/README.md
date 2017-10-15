# google-oauth-handler

Package `google_oauth_handler` transparently handles OAuth authentication with
Google.

Create an Authenticator and then insert it as middleware in front of any
resources you want to protect behind Google login, via authenticator.Handle.
Handle will call the next middleware with (w, r, *Auth), which you can use
to make requests to the Google API.

The Authenticator handles the OAuth workflow for you, redirecting users to
Google, handling the callback and setting an encrypted cookie in the user's
browser.

For more information, see the [godoc documentation][godoc].

[godoc]: https://godoc.org/github.com/kevinburke/google-oauth-handler

### Example

```go
package google_oauth_handler_test

import (
	"encoding/hex"
	"fmt"
	"net/http"

	google "github.com/kevinburke/google-oauth-handler"
	"golang.org/x/oauth2"
)

var key *[32]byte

func init() {
	secretKeyBytes, _ := hex.DecodeString("982a732cc3d72d13678dee2609cf55d736711ff1f293f95cab41bd45e5d77870")
	key = new([32]byte)
	copy(key[:], secretKeyBytes)
}

func Example() {
	cfg := google.Config{
		SecretKey: key,
		BaseURL:   "https://example.com",
		ClientID:  "customdomain.apps.googleusercontent.com",
		Secret:    "W-secretkey",
		Scopes: []string{
			"email",
			"https://www.googleapis.com/auth/gmail.send",
		},
	}
	auth := google.NewAuthenticator(cfg)
	http.Handle("/", auth.Handle(func(w http.ResponseWriter, r *http.Request, auth *google.Auth) {
		fmt.Fprintf(w, "<html><body><h1>Hello World</h1><p>Token: %s</p></body></html>", auth.AccessToken)
	}))
}
```
