package main

// Helper functions for setting a flash message as a cookie, and then reading
// that flash message in another request.

import (
	"net/http"
	"strings"
	"time"
)

// FlashSuccess encrypts msg and sets it as a cookie on w. Only one success
// message can be set on w; the last call to FlashSuccess will be set on the
// response.
func FlashSuccess(w http.ResponseWriter, msg string, key *[32]byte) {
	setCookie(w, msg, "flash-success", key)
}

// FlashError encrypts msg and sets it as a cookie on w. Only one error can be
// set on w; the last call to FlashError will be set on the response.
func FlashError(w http.ResponseWriter, msg string, key *[32]byte) {
	setCookie(w, msg, "flash-error", key)
}

func setCookie(w http.ResponseWriter, msg string, name string, key *[32]byte) {
	c := &http.Cookie{
		Name:     name,
		Path:     "/",
		Value:    opaque(name+"|"+msg, key),
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

// GetFlashSuccess finds a flash success message in the request (if one exists).
// If one exists then it's unset and returned.
func GetFlashSuccess(w http.ResponseWriter, r *http.Request, key *[32]byte) string {
	return getCookie(w, r, "flash-success", key)
}

// GetFlashError finds a flash error in the request (if one exists). If one
// exists then it's unset and returned.
func GetFlashError(w http.ResponseWriter, r *http.Request, key *[32]byte) string {
	return getCookie(w, r, "flash-error", key)
}

func getCookie(w http.ResponseWriter, r *http.Request, name string, key *[32]byte) string {
	cookie, err := r.Cookie(name)
	if err == http.ErrNoCookie {
		return ""
	}
	clearCookie(w, name)
	msg, err := unopaque(cookie.Value, key)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(msg, name+"|") {
		clearCookie(w, name)
		return ""
	}
	return msg[len(name)+1:]
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(1, 0),
	})
}
