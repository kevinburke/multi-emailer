package main

import (
	"net/http"
	"time"
)

func FlashError(w http.ResponseWriter, msg string, key *[32]byte) {
	c := &http.Cookie{
		Name:     "flash-error",
		Path:     "/",
		Value:    opaque(msg, key),
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

// GetFlashError finds a flash error in the request (if one exists). If one
// exists then it's unset and returned.
func GetFlashError(w http.ResponseWriter, r *http.Request, key *[32]byte) string {
	cookie, err := r.Cookie("flash-error")
	if err == http.ErrNoCookie {
		return ""
	}
	clearCookie(w, "flash-error")
	msg, err := unopaque(cookie.Value, key)
	if err != nil {
		return ""
	}
	return msg
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(1, 0),
	})
}
