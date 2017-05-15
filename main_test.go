package main

import (
	"net/http/httptest"
	"testing"

	google "github.com/kevinburke/google-oauth-handler"
)

func TestServerRedirects(t *testing.T) {
	mux := NewServeMux(google.NewAuthenticator(google.Config{
		SecretKey: NewRandomKey(),
	}), nil, "", false, "")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /: got code %d, want 200", w.Code)
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
