//go:build !prod

package web

import (
	"net/http/httputil"
	"net/url"
)

func HandleWeb() *httputil.ReverseProxy {
	targetURL, _ := url.Parse("http://localhost:3000")
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	return proxy
}
