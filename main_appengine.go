// +build appengine

package main

import (
	"net/http"

	"github.com/kevinburke/rest"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func init() {
	rest.RegisterHandler(500, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := appengine.NewContext(r)
		err := rest.CtxErr(r)
		log.Errorf(ctx, "%s %s: %v", r.Method, r.URL.Path, err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(500)
		w.Write([]byte("<html><body>Server Error</body></html>"))
	}))
	_, mux := commonMain()
	http.Handle("/", mux)
}

func main() {
	appengine.Main()
}
