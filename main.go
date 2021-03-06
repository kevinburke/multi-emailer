// multi-emailer is a tool for sending many emails at once.
//
// They'll be sent from your personal Gmail account, and the recipient's name
// will be attached to the top of each email, so it looks like you hand wrote it
// (unless you inspect the email *very* closely).
//
// When users visit the site they'll be redirected to a Google approval page.
// This page will ask them for permission to send emails on their behalf. Then
// they'll be redirected and can type away!
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	google "github.com/kevinburke/google-oauth-handler"
	"github.com/kevinburke/handlers"
	"github.com/kevinburke/multi-emailer/assets"
	"github.com/kevinburke/rest"
	gmail "google.golang.org/api/gmail/v1"
	yaml "gopkg.in/yaml.v2"
)

var idRxPart = "[-_a-z0-9A-Z]+"

// GET /
// GET /id
var homeRx = regexp.MustCompile(fmt.Sprintf(`^/(%s)?$`, idRxPart))

// GET /<id>/recipients
var recipientsRx = regexp.MustCompile(fmt.Sprintf(`^/(%s)/recipients$`, idRxPart))
var validIDRx = regexp.MustCompile(fmt.Sprintf(`^%s$`, idRxPart))

var logger log.Logger

var homepageTpl *template.Template

func init() {
	logger = handlers.Logger
	homepageHTML := assets.MustAssetString("templates/index.html")
	homepageTpl = template.Must(template.New("homepage").Parse(homepageHTML))
}

var goVersion = runtime.Version()

const Version = "1.7"

var DefaultPort = 8048

// Static file HTTP server; all assets are packaged up in the assets directory
// with go-bindata.
type static struct {
	modTime time.Time
}

func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		r.URL.Path = "/static/favicon.ico"
	}
	if r.URL.Path == "/privacy" {
		r.URL.Path = "/static/privacy.html"
	}
	if r.URL.Path == "/terms-of-service" {
		r.URL.Path = "/static/license.txt"
	}
	bits, err := assets.Asset(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		rest.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, r.URL.Path, s.modTime, bytes.NewReader(bits))
}

func render(w http.ResponseWriter, r *http.Request, tpl *template.Template, name string, data interface{}) {
	buf := new(bytes.Buffer)
	if err := tpl.ExecuteTemplate(buf, name, data); err != nil {
		rest.ServerError(w, r, err)
		return
	}
	w.Write(buf.Bytes())
}

func logout(auth *google.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth.Logout(w)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

type homepageData struct {
	Email      *mail.Address
	Groups     map[string]*Group
	Error      string
	Success    string
	Title      string
	Version    string
	PublicHost string
	Subject    string
	Body       string
	IsHomepage bool
	// If there's only one group and one recipient, put opening line there
	OpeningLine string
	AuthURL     string
}

func NewServeMux(authenticator *google.Authenticator, mailer *Mailer, title string, withGoogle bool, publicHost string, siteVerification string) http.Handler {
	staticServer := &static{
		modTime: time.Now().UTC(),
	}
	if mailer == nil {
		mailer = &Mailer{
			secretKey: NewRandomKey(),
		}
	}

	renderRecipients := func(w http.ResponseWriter, r *http.Request) {
		match := recipientsRx.FindStringSubmatch(r.URL.Path)
		group, ok := mailer.Groups[match[1]]
		if !ok {
			rest.NotFound(w, r)
			return
		}
		data, err := yaml.Marshal(group.Recipients)
		if err != nil {
			rest.ServerError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	}

	renderHomepage := func(w http.ResponseWriter, r *http.Request, email *mail.Address, authURL string) {
		push(w, "/static/bootstrap.min.css", "style")
		push(w, "/static/style.css", "style")
		vals := r.URL.Query()
		if vals.Get("subject") != "" || vals.Get("body") != "" {
			setCookie(w, vals.Get("subject"), "subject", mailer.secretKey)
			setCookie(w, vals.Get("body"), "body", mailer.secretKey)
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
			return
		}
		subjCookie := getCookie(w, r, "subject", mailer.secretKey, email != nil)
		bodyCookie := getCookie(w, r, "body", mailer.secretKey, email != nil)
		match := homeRx.FindStringSubmatch(r.URL.Path)
		var groups map[string]*Group
		if match == nil || match[1] == "" {
			groups = mailer.Groups
		} else {
			if group, ok := mailer.Groups[match[1]]; !ok {
				rest.NotFound(w, r)
				return
			} else {
				groups = make(map[string]*Group)
				groups[match[1]] = group
			}
		}
		var openingLine string
		if len(groups) == 1 {
			for k := range groups {
				if len(groups[k].Recipients) == 1 {
					openingLine = groups[k].Recipients[0].OpeningLine
				}
				break
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(w, r, homepageTpl, "homepage", &homepageData{
			Title:       title,
			Email:       email,
			Groups:      groups,
			Error:       GetFlashError(w, r, mailer.secretKey),
			Success:     GetFlashSuccess(w, r, mailer.secretKey),
			Version:     goVersion,
			PublicHost:  publicHost,
			Subject:     subjCookie,
			Body:        bodyCookie,
			IsHomepage:  r.URL.Path == "/",
			OpeningLine: openingLine,
			AuthURL:     authURL,
		})
	}

	r := new(handlers.Regexp)

	r.Handle(regexp.MustCompile(`(^/static|^/favicon.ico$|^/privacy$|^/terms-of-service$)`), []string{"GET"}, handlers.GZip(staticServer))
	if siteVerification != "" {
		r.HandleFunc(regexp.MustCompile("/"+regexp.QuoteMeta(siteVerification)), []string{"GET"}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, "google-site-verification: "+siteVerification)
		})
	}
	r.HandleFunc(recipientsRx, []string{"GET"}, func(w http.ResponseWriter, r *http.Request) {
		renderRecipients(w, r)
	})
	if withGoogle {
		r.HandleFunc(regexp.MustCompile(`^/logout$`), []string{"POST"}, logout(authenticator))
		authenticator.SetLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vals := r.URL.Query()
			if vals.Get("subject") != "" || vals.Get("body") != "" {
				setCookie(w, vals.Get("subject"), "subject", mailer.secretKey)
				setCookie(w, vals.Get("body"), "body", mailer.secretKey)
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return
			}
			u := authenticator.URL(r)
			renderHomepage(w, r, nil, u)
		}))
		r.Handle(homeRx, []string{"GET"}, authenticator.Handle(func(w http.ResponseWriter, r *http.Request, auth *google.Auth) {
			renderHomepage(w, r, auth.Email, "")
		}))
		r.Handle(regexp.MustCompile(`^/auth/callback$`), []string{"GET"}, authenticator.Handle(func(w http.ResponseWriter, r *http.Request, _ *google.Auth) {
			http.Redirect(w, r, "/", http.StatusFound)
		}))
		r.Handle(regexp.MustCompile(`^/v1/send$`), []string{"POST"}, authenticator.Handle(mailer.sendMail))
	} else {
		// For testing; no authentication.
		testEmail, _ := mail.ParseAddress("Test Email <test@example.org>")
		r.HandleFunc(homeRx, []string{"GET"}, func(w http.ResponseWriter, r *http.Request) {
			renderHomepage(w, r, testEmail, "")
		})
	}
	// for Google App Engine
	r.HandleFunc(regexp.MustCompile(`^/_ah/health$`), []string{"GET"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok")
	})
	return r
}

type ConfigGroup struct {
	ID         string             `yaml:"id"`
	Name       string             `yaml:"name"`
	Recipients []*ConfigRecipient `yaml:"recipients"`
}

type ConfigRecipient struct {
	Email       string   `yaml:"email"`
	CC          []string `yaml:"cc,omitempty"`
	OpeningLine string   `yaml:"opening_line"`
}

type FileConfig struct {
	SecretKey      string         `yaml:"secret_key"`
	PublicHost     string         `yaml:"public_host"`
	HTTPOnly       bool           `yaml:"http_only"`
	GoogleClientID string         `yaml:"google_client_id"`
	GoogleSecret   string         `yaml:"google_secret"`
	Groups         []*ConfigGroup `yaml:"groups"`
	Port           *int           `yaml:"port"`
	Title          string         `yaml:"title"`

	// For development; ignore Google authentication.
	NoGoogleAuth bool `yaml:"no_google_auth"`

	// For TLS configuration.
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`

	// Should be a string like "google4f9d0c78202b2454.html". If non-empty and
	// not starting with "google", it will be prepended. If it does not end with
	// ".html", ".html" will be appended.
	GoogleSiteVerification string `yaml:"google_site_verification"`
}

var check = flag.Bool("check", false, "Validate the config file and then exit")
var cfg = flag.String("config", "config.yml", "Path to a config file")
var errWrongLength = errors.New("secret key has wrong length; should be a 64-byte hex string")

// NewRandomKey returns a random key or panics if one cannot be provided.
func NewRandomKey() *[32]byte {
	key := new([32]byte)
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		panic(err)
	}
	return key
}

// getSecretKey produces a valid [32]byte secret key or returns an error. If
// hexKey is the empty string, a valid 32 byte key will be randomly generated
// and returned. If hexKey is invalid, an error is returned.
func getSecretKey(hexKey string) (*[32]byte, error) {
	if hexKey == "" {
		return NewRandomKey(), nil
	}

	if len(hexKey) != 64 {
		return nil, errWrongLength
	}
	secretKeyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	secretKey := new([32]byte)
	copy(secretKey[:], secretKeyBytes)
	return secretKey, nil
}

func validID(id string) bool {
	return validIDRx.MatchString(id)
}

func loadConfig(filename string) (*FileConfig, error) {
	data, err := ioutil.ReadFile(*cfg)
	if err != nil {
		return nil, err
	}
	c := new(FileConfig)
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

func commonMain() (*FileConfig, http.Handler) {
	flag.Parse()
	if flag.NArg() > 2 {
		os.Stderr.WriteString("too many arguments")
		os.Exit(2)
	}
	c, err := loadConfig(*cfg)
	if err != nil {
		logger.Error("Error loading/parsing config file", "err", err)
		os.Exit(2)
	}
	if *check {
		os.Exit(0)
	}
	key, err := getSecretKey(c.SecretKey)
	if err != nil {
		logger.Error("Error getting secret key", "err", err)
		os.Exit(2)
	}
	m := &Mailer{Groups: make(map[string]*Group), Logger: logger, secretKey: key}
	for _, group := range c.Groups {
		if group.ID == "" {
			logger.Error("Please provide a group ID")
			os.Exit(2)
		}
		if !validID(group.ID) {
			logger.Error("Invalid group ID, stick to numbers and letters", "id", group.ID)
			os.Exit(2)
		}
		if group.Name == "" {
			group.Name = group.ID
		}
		recs := make([]*Recipient, len(group.Recipients))
		for i, recipient := range group.Recipients {
			addr, err := mail.ParseAddress(recipient.Email)
			if err != nil {
				logger.Error("Could not parse email address", "err", err, "email", recipient.Email)
				os.Exit(2)
			}
			if recipient.OpeningLine == "" {
				recipient.OpeningLine = "To whom it may concern"
			}
			ccs := make([]mail.Address, len(recipient.CC))
			for i := range recipient.CC {
				ccaddr, err := mail.ParseAddress(recipient.CC[i])
				if err != nil {
					logger.Error("Could not parse cc email address", "err", err, "email", recipient.CC[i])
					os.Exit(2)
				}
				ccs[i] = *ccaddr
			}
			recs[i] = &Recipient{
				Address:     *addr,
				CC:          ccs,
				OpeningLine: recipient.OpeningLine,
			}
		}
		m.Groups[group.ID] = &Group{
			ID:         group.ID,
			Name:       group.Name,
			Recipients: recs,
		}
	}
	if c.Port == nil {
		port, ok := os.LookupEnv("PORT")
		if ok {
			portStr, err := strconv.Atoi(port)
			if err != nil {
				logger.Error("Invalid port", "err", err, "port", port)
				os.Exit(2)
			}
			c.Port = &portStr
		} else {
			c.Port = &DefaultPort
		}
	}

	var host string
	if c.PublicHost != "" {
		u, err := url.Parse(c.PublicHost)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(2)
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		host = u.String()
	} else {
		host = "http://localhost:" + strconv.Itoa(*c.Port)
	}
	gcfg := google.Config{
		SecretKey:               key,
		BaseURL:                 host,
		AllowUnencryptedTraffic: true,
		ClientID:                c.GoogleClientID,
		Secret:                  c.GoogleSecret,
		Scopes: []string{
			"email",
			gmail.GmailSendScope,
		},
	}
	if c.GoogleSiteVerification != "" {
		if !strings.HasPrefix(c.GoogleSiteVerification, "google") {
			c.GoogleSiteVerification = "google" + c.GoogleSiteVerification
		}
		if !strings.HasSuffix(c.GoogleSiteVerification, ".html") {
			c.GoogleSiteVerification = c.GoogleSiteVerification + ".html"
		}
	}
	authenticator := google.NewAuthenticator(gcfg)
	mux := NewServeMux(authenticator, m, c.Title, !c.NoGoogleAuth, c.PublicHost, c.GoogleSiteVerification)
	mux = handlers.UUID(mux)
	if strings.HasPrefix(c.PublicHost, "https://") {
		mux = handlers.RedirectProto(mux)
		mux = handlers.STS(mux)
	}
	mux = handlers.Server(mux, "multi-emailer/"+Version)
	mux = handlers.Debug(mux)
	mux = handlers.Log(mux)
	mux = handlers.Duration(mux)
	return c, mux
}
