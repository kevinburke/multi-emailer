// Package google_oauth_handler transparently handles OAuth authentication with
// Google.
//
// Create an Authenticator and then insert it as middleware in front of any
// resources you want to protect behind Google login, via authenticator.Handle.
// Handle will call the next middleware with (w, r, *Token), which you can use
// to make requests to the Google API.
//
// The Authenticator handles the OAuth workflow for you, redirecting users to
// Google, handling the callback and setting an encrypted cookie in the user's
// browser.
package google_oauth_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const Version = "0.4"

// DefaultExpiry is the duration of a valid cookie.
var DefaultExpiry = 14 * 24 * time.Hour

// AuthTimeout is the amount of time to allow users to complete an
// authentication attempt.
var AuthTimeout = 1 * time.Hour

// Timeout is the amount of time to wait for a response from Google when asking
// for information.
var Timeout = 30 * time.Second

const cookieName = "google-oauth-token"

// Config allows users to configure an Authenticator.
type Config struct {
	// The logger to use for logging errors in the authentication workflow.
	Logger log.Logger
	// BaseURL is the server's base URL (scheme+host), used to set cookies.
	BaseURL string
	// SecretKey is used to encrypt and decrypt the OAuth nonce and cookies.
	SecretKey *[32]byte

	// The Google ClientID and Secret. To figure out how to get these, visit
	// https://github.com/saintpete/logrole/blob/master/docs/google.md
	ClientID string
	Secret   string

	// List of scopes (a.k.a permissions) to ask for.
	// For a list of valid scopes see https://developers.google.com/identity/protocols/googlescopes.
	Scopes []string
	// Set to "false" to set Secure: false on cookies.
	AllowUnencryptedTraffic bool
	// If non-empty, limit access to users with email addresses in these domains.
	AllowedDomains []string
	// ServeLogin will be called when the user needs to authenticate with
	// Google, for example if you want to display a custom login page with
	// a "Log in with Google" button.
	//
	// If ServeLogin is nil, all users that need to authenticate will be
	// immediately redirected to Google.
	ServeLogin http.Handler
	// Which URL Google should hit when they redirect users back to your site
	// after logging in. Defaults to "/auth/callback".
	CallbackPath string
}

// Auth is returned in the callback to Handle().
type Auth struct {
	// The Google email address for this user.
	Email *mail.Address
	// A Client that's configured to use the Google credentials
	Client *http.Client
	Token  *oauth2.Token
}

// An Authenticator transparently handles authentication with Google. Create an
// Authenticator by calling NewAuthenticator(Config{}).
type Authenticator struct {
	logger                  log.Logger
	conf                    *oauth2.Config
	allowUnencryptedTraffic bool
	callback                string
	allowedDomains          []string
	secretKey               *[32]byte

	// Login gets called when the user must authenticate. The default Login
	// behavior is to redirect to the Google authentication page.
	login   http.Handler
	loginMu sync.Mutex
}

// SetLogin sets the login handler to h. A convenience handler since this may be
// configured later than the other parts of the google oauth handler's config.
func (a *Authenticator) SetLogin(h http.Handler) {
	a.loginMu.Lock()
	a.login = h
	a.loginMu.Unlock()
}

// NewAuthenticator creates a new Authenticator that can
// authenticate requests via Google login.
//
// To get a clientID and clientSecret, see
// https://github.com/saintpete/logrole/blob/master/docs/google.md.
func NewAuthenticator(c Config) *Authenticator {
	if c.CallbackPath == "" {
		c.CallbackPath = "/auth/callback"
	}
	conf := &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.Secret,
		RedirectURL:  c.BaseURL + c.CallbackPath,
		// https://developers.google.com/identity/protocols/googlescopes#google_sign-in
		Scopes:   c.Scopes,
		Endpoint: google.Endpoint,
	}
	if c.Logger == nil {
		c.Logger = handlers.Logger
	}
	a := &Authenticator{
		logger: c.Logger,
		conf:   conf,
		allowUnencryptedTraffic: c.AllowUnencryptedTraffic,
		allowedDomains:          c.AllowedDomains,
		secretKey:               c.SecretKey,
		callback:                c.CallbackPath,
	}
	// no need to lock since no one else can call till it's returned
	if c.ServeLogin == nil {
		a.login = http.HandlerFunc(a.defaultLogin)
	} else {
		a.login = c.ServeLogin
	}
	return a
}

// URL returns a link to the Google auth URL for this Authenticator. If
// the http.Request contains a query parameter named "g", the user will be
// redirected to that page upon returning from Google.
func (a *Authenticator) URL(r *http.Request) string {
	var uri string
	if g := r.URL.Query().Get("g"); g != "" {
		// prevent open redirect by only using the Path part
		u, err := url.Parse(g)
		if err == nil {
			uri = u.Path
		} else {
			uri = r.URL.RequestURI()
		}
	} else {
		uri = r.URL.RequestURI()
	}
	st := state{
		CurrentURL: uri,
		Time:       time.Now().UTC(),
	}
	bits, err := json.Marshal(st)
	if err != nil {
		panic(err)
	}
	encoded := opaqueByte(bits, a.secretKey)
	return a.conf.AuthCodeURL(encoded)
}

// defaultLogin redirects the request to the Google authentication URL.
func (a *Authenticator) defaultLogin(w http.ResponseWriter, r *http.Request) {
	u := a.URL(r)
	http.Redirect(w, r, u, http.StatusFound)
}

func (a *Authenticator) validState(encrypted string) (string, bool) {
	b, err := unopaqueByte(encrypted, a.secretKey)
	if err != nil {
		return "", false
	}
	st := new(state)
	if err := json.Unmarshal(b, st); err != nil {
		return "", false
	}
	if time.Since(st.Time) > AuthTimeout {
		return "", false
	}
	return st.CurrentURL, true
}

// Handle returns an http.Handler that only calls f if the request carries
// a valid authentication cookie. If not, a.Login is called, to trigger an
// authentication workflow with Google. If the user approves access for the
// provided scopes, we set an encrypted cookie on the request and call f(w, r,
// token). The token can then be used to make requests to Google's APIs.
//
// Handle must handle requests to the CallbackPath (which defaults to
// /auth/callback) in order to set a valid cookie.
func (a *Authenticator) Handle(f func(http.ResponseWriter, *http.Request, *Auth)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == a.callback {
			a.handleGoogleCallback(w, r)
			return
		}
		a.loginMu.Lock()
		login := a.login
		a.loginMu.Unlock()
		// Check if the request has a valid cookie, if not redirect to login.
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			login.ServeHTTP(w, r)
			return
		}
		val, err := unopaqueByte(cookie.Value, a.secretKey)
		if err != nil {
			// Bad cookie. TODO need a 400 bad request here
			login.ServeHTTP(w, r)
			return
		}
		t := new(token)
		if err := json.Unmarshal(val, t); err != nil {
			// TODO clear it
			login.ServeHTTP(w, r)
			return
		}
		now := time.Now().UTC()
		if t.Expiry.Before(now) {
			// TODO logout
			login.ServeHTTP(w, r)
			return
		}
		// It's possible the AccessToken has expired by the time the user makes
		// a request. We don't want to log them out, so try to refresh the token
		// now if necessary.
		src := a.conf.TokenSource(wctx(r), t.Token)
		newToken, err := src.Token()
		if err != nil {
			// some sort of error getting a token, ask user to log in again.
			login.ServeHTTP(w, r)
			return
		}
		if t.Token.AccessToken != newToken.AccessToken {
			// we got a new token
			t.Token = newToken
			cookie := a.newCookie(t)
			http.SetCookie(w, cookie)
		}

		// todo - not super happy with this.
		f(w, r, &Auth{
			t.Email,
			a.conf.Client(wctx(r), t.Token),
			t.Token,
		})
	})
}

func (a *Authenticator) handleGoogleCallback(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	st := query.Get("state")
	currentURL, ok := a.validState(st)
	if !ok {
		http.Redirect(w, r, "/", 302)
		return errors.New("invalid state")
	}
	code := query.Get("code")
	if code == "" {
		a.logger.Warn("Callback request has valid state, no code")
		http.Redirect(w, r, "/", 302)
		return errors.New("invalid state")
	}
	ctx, cancel := context.WithTimeout(wctx(r), Timeout)
	defer cancel()
	tok, err := a.conf.Exchange(ctx, code)
	if err != nil {
		// TODO this can return 400+JSON if you try to redeem a code twice:
		// Response: {
		//  "error" : "invalid_grant",
		//  "error_description" : "Invalid code."
		// }
		rest.ServerError(w, r, err)
		return err
	}

	client := a.conf.Client(ctx, tok)
	u, err := getGoogleUserData(ctx, client)
	if err != nil {
		rest.ServerError(w, r, err)
		return err
	}
	// TODO verify allowed domains.
	t := newToken(u.Address(), tok)
	cookie := a.newCookie(t)
	http.SetCookie(w, cookie)
	http.Redirect(w, r, currentURL, 302)
	return errors.New("redirected, make another request")
}

func (a *Authenticator) newCookie(t *token) *http.Cookie {
	b, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	text := opaqueByte(b, a.secretKey)
	return &http.Cookie{
		Name:     cookieName,
		Value:    text,
		Path:     "/",
		Secure:   a.allowUnencryptedTraffic == false,
		Expires:  t.Expiry,
		HttpOnly: true,
	}
}

type token struct {
	Email  *mail.Address
	Token  *oauth2.Token
	Expiry time.Time
}

type state struct {
	CurrentURL string
	Time       time.Time
}

func newToken(id *mail.Address, tok *oauth2.Token) *token {
	return &token{
		Email:  id,
		Token:  tok,
		Expiry: time.Now().UTC().Add(DefaultExpiry),
	}
}

// Base URL to get user data from.
var userDataBase = "https://www.googleapis.com"

// Path that allows you to get user data.
var userDataPath = "/oauth2/v3/userinfo"

// The data about users that we get back from Google.
type googleUser struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Profile       string `json:"profile"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Gender        string `json:"gender"`
	Locale        string `json:"locale"`
	HD            string `json:"hd"`
}

func (g *googleUser) Address() *mail.Address {
	return &mail.Address{
		Name:    g.Name,
		Address: g.Email,
	}
}

func getGoogleUserData(ctx context.Context, client *http.Client) (*googleUser, error) {
	if client == nil {
		client = http.DefaultClient
	}
	rc := rest.NewClient("", "", userDataBase)
	rc.Client = client
	req, err := rc.NewRequest("GET", userDataPath, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	u := new(googleUser)
	err = rc.Do(req, u)
	if err != nil {
		return nil, err
	}
	if u.Email == "" {
		return nil, fmt.Errorf("No email address for user: %s", u.Name)
	}
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return nil, err
	}
	if u.EmailVerified == false {
		return nil, fmt.Errorf("User %s does not have a verified email address", u.Name)
	}
	return u, err
}
