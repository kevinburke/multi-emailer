package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
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

var homepageTpl *template.Template

func init() {
	homepageHTML := assets.MustAssetString("templates/index.html")
	homepageTpl = template.Must(template.New("homepage").Parse(homepageHTML))
}

const Version = "0.5"
const DefaultPort = 8048

// Static file HTTP server; all assets are packaged up in the assets directory
// with go-bindata.
type static struct {
	modTime time.Time
}

func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		r.URL.Path = "/static/favicon.ico"
	}
	bits, err := assets.Asset(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		rest.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, r.URL.Path, s.modTime, bytes.NewReader(bits))
}

func render(w http.ResponseWriter, tpl *template.Template, name string, data interface{}) {
	buf := new(bytes.Buffer)
	if err := tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), 500)
	}
	w.Write(buf.Bytes())
}

func NewServeMux(authenticator *google.Authenticator, mailer *Mailer) http.Handler {
	staticServer := &static{
		modTime: time.Now().UTC(),
	}

	r := new(handlers.Regexp)
	r.Handle(regexp.MustCompile(`(^/static|^/favicon.ico$)`), []string{"GET"}, handlers.GZip(staticServer))
	r.Handle(regexp.MustCompile(`^/$`), []string{"GET"}, authenticator.Handle(func(w http.ResponseWriter, r *http.Request, auth *google.Auth) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(w, homepageTpl, "homepage", struct {
			Email   *mail.Address
			Groups  map[string]*Group
			Error   string
			Success string
		}{
			Email:   auth.Email,
			Groups:  mailer.Groups,
			Error:   GetFlashError(w, r, mailer.secretKey),
			Success: GetFlashSuccess(w, r, mailer.secretKey),
		})
	}))
	r.Handle(regexp.MustCompile(`^/auth/callback$`), []string{"GET"}, authenticator.Handle(func(w http.ResponseWriter, r *http.Request, _ *google.Auth) {
		http.Redirect(w, r, "/", 302)
	}))
	r.Handle(regexp.MustCompile(`^/v1/send$`), []string{"POST"}, authenticator.Handle(mailer.sendMail))
	// for Google App Engine
	r.HandleFunc(regexp.MustCompile(`^/_ah/health$`), []string{"GET"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "ok")
	})
	// Add more routes here.
	return r
}

var logger log.Logger

func init() {
	logger = handlers.Logger
}

type ConfigGroup struct {
	ID         string             `yaml:"id"`
	Name       string             `yaml:"name"`
	Recipients []*ConfigRecipient `yaml:"recipients"`
}

type ConfigRecipient struct {
	Email       string `yaml:"email"`
	OpeningLine string `yaml:"opening_line"`
}

type FileConfig struct {
	SecretKey      string         `yaml:"secret_key"`
	PublicHost     string         `yaml:"public_host"`
	GoogleClientID string         `yaml:"google_client_id"`
	GoogleSecret   string         `yaml:"google_secret"`
	Groups         []*ConfigGroup `yaml:"groups"`
	Port           int            `yaml:"port"`
}

var cfg = flag.String("config", "config.yml", "Path to a config file")
var errWrongLength = errors.New("Secret key has wrong length. Should be a 64-byte hex string")

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

func main() {
	flag.Parse()
	if flag.NArg() > 2 {
		os.Stderr.WriteString("too many arguments")
		os.Exit(2)
	}

	data, err := ioutil.ReadFile(*cfg)
	c := new(FileConfig)
	if err == nil {
		if err := yaml.Unmarshal(data, c); err != nil {
			logger.Error("Couldn't parse config file", "err", err)
			os.Exit(2)
		}
	} else {
		logger.Error("Couldn't find config file", "err", err)
		os.Exit(2)
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
			recs[i] = &Recipient{
				Address:     addr,
				OpeningLine: recipient.OpeningLine,
			}
		}
		m.Groups[group.ID] = &Group{
			ID:         group.ID,
			Name:       group.Name,
			Recipients: recs,
		}
	}

	if c.Port == 0 {
		port, ok := os.LookupEnv("PORT")
		if ok {
			c.Port, err = strconv.Atoi(port)
			if err != nil {
				logger.Error("Invalid port", "err", err, "port", port)
				os.Exit(2)
			}
		} else {
			c.Port = DefaultPort
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
		host = "http://localhost:" + strconv.Itoa(c.Port)
	}
	cfg := google.Config{
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
	authenticator := google.NewAuthenticator(cfg)
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(c.Port))
	if err != nil {
		logger.Error(err.Error())
		os.Exit(2)
	}
	logger.Info("Started server", "port", c.Port)
	mux := NewServeMux(authenticator, m)
	mux = handlers.UUID(mux)
	if strings.HasPrefix(c.PublicHost, "https://") {
		mux = handlers.RedirectProto(mux)
		mux = handlers.STS(mux)
	}
	mux = handlers.Server(mux, "multi-emailer/"+Version)
	mux = handlers.Debug(mux)
	mux = handlers.Log(mux)
	mux = handlers.Duration(mux)
	http.Serve(ln, mux)
}
