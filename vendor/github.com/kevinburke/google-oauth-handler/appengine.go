// +build appengine

package google_oauth_handler

import (
	"context"
	"net/http"

	"google.golang.org/appengine"
)

func wctx(r *http.Request) context.Context {
	return appengine.NewContext(r)
}
