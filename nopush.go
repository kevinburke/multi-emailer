// +build !go1.8

package main

import (
	"fmt"
	"net/http"
)

// Push the given resource to the client. Destination is a "request destination"
// per this spec: https://fetch.spec.whatwg.org/#concept-request-destination.
func push(w http.ResponseWriter, resource string, destination string) {
	// HTTP2 push is only supported in Go 1.8 and up; implement the preload spec
	// in case there's a proxy that supports HTTP2.
	// https://w3c.github.io/preload/#server-push-http-2
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=preload; as=%s", resource, destination))
}
