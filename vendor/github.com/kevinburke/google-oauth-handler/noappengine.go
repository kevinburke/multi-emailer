// +build !appengine

package google_oauth_handler

import (
	"context"
	"net/http"
)

func wctx(r *http.Request) context.Context {
	return r.Context()
}
