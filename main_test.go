package main

import (
	"net/http/httptest"
	"net/mail"
	"strings"
	"testing"

	google "github.com/kevinburke/google-oauth-handler"
)

func TestServerReturns200(t *testing.T) {
	t.Parallel()
	mux := NewServeMux(google.NewAuthenticator(google.Config{
		SecretKey: NewRandomKey(),
	}), nil, "", false, "", "")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /: got code %d, want 200", w.Code)
	}
}

func TestSiteVerification(t *testing.T) {
	t.Parallel()
	mux := NewServeMux(google.NewAuthenticator(google.Config{
		SecretKey: NewRandomKey(),
	}), nil, "", false, "", "google4f9d0c78202b2454.html")
	req := httptest.NewRequest("GET", "/google4f9d0c78202b2454.html", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /google4f9d0c78202b2454.html: got code %d, want 200", w.Code)
	}
	want := "google-site-verification: google4f9d0c78202b2454.html"
	if b := w.Body.String(); b != want {
		t.Errorf("site verification: got %s, want %s", b, want)
	}
}

func TestPrivacyPolicy(t *testing.T) {
	t.Parallel()
	mux := NewServeMux(google.NewAuthenticator(google.Config{
		SecretKey: NewRandomKey(),
	}), nil, "", false, "", "")
	req := httptest.NewRequest("GET", "/privacy", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /privacy: got code %d, want 200", w.Code)
	}
	if b := w.Body.String(); !strings.Contains(b, "<h1>Privacy Policy</h1>") {
		t.Errorf("privacy policy: should see <h1>Privacy</h1>, got %s", b)
	}
}

var group *Group

func init() {
	addr, _ := mail.ParseAddress("Recipient <recipient@example.com>")
	cc, _ := mail.ParseAddress("CC <cc@example.com>")
	group = &Group{
		Recipients: []*Recipient{{*addr, []mail.Address{*cc}, "Dear Test Group"}},
		ID:         "test-group-slug",
		Name:       "Test Group Slug",
	}
}

func TestRecipients(t *testing.T) {
	mailer := &Mailer{Groups: map[string]*Group{
		"test-group-slug": group,
	}}
	mux := NewServeMux(google.NewAuthenticator(google.Config{
		SecretKey: NewRandomKey(),
	}), mailer, "", false, "", "")
	req := httptest.NewRequest("GET", "/test-group-slug/recipients", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /test-group-slug: got code %d, want 200", w.Code)
	}
	want := `- address:
    name: Recipient
    address: recipient@example.com
  cc:
  - name: CC
    address: cc@example.com
  opening_line: Dear Test Group
`
	if b := w.Body.String(); b != want {
		t.Errorf("recipients: should be\n%q\n, got\n%q\n", want, b)
	}
}

var idTests = []struct {
	in   string
	want bool
}{
	{"foo", true},
	{"-_--9-_", true},
	{"$", false},
	{"", false},
	{"#haeuteeu", false},
}

func TestValidID(t *testing.T) {
	t.Parallel()
	for _, tt := range idTests {
		got := validID(tt.in)
		if got != tt.want {
			t.Errorf("validID(%q): got %t, want %t", tt.in, got, tt.want)
		}
	}
}
