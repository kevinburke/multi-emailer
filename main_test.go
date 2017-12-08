package main

import (
	"net/http/httptest"
	"testing"

	google "github.com/kevinburke/google-oauth-handler"
)

func TestServerReturns200(t *testing.T) {
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
	for _, tt := range idTests {
		got := validID(tt.in)
		if got != tt.want {
			t.Errorf("validID(%q): got %t, want %t", tt.in, got, tt.want)
		}
	}
}
