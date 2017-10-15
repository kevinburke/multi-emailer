package google_oauth_handler_test

import (
	"fmt"
	"net/http"

	google "github.com/kevinburke/google-oauth-handler"
)

func ExampleAuthenticator_URL() {
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
	r, _ := http.NewRequest("GET", "/", nil)
	fmt.Println(auth.URL(r)) // "https://accounts.google.com/o/oauth2/..."
}
