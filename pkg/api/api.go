package api

import (
	"net"
	"net/http"
	"time"
)

const RustAPIReference = "github.com/PeterPonyu/emberforge-go/pkg/api"

// newStreamingHTTPClient returns an *http.Client suitable for SSE / chunked
// streaming endpoints. A blanket Client.Timeout would cancel the response body
// mid-stream, killing long-running generations (e.g. thinking models). Only
// Transport-level timeouts are set to guard against hangs on connection setup:
//
//   - DialContext timeout:       10 s — TCP connect
//   - TLSHandshakeTimeout:       10 s — TLS negotiation
//   - ResponseHeaderTimeout:     60 s — wait for the first response header byte
//
// The response body is left uncapped so streams complete without an overall
// deadline cutting them short.
func newStreamingHTTPClient() *http.Client {
	t := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	return &http.Client{Transport: t}
}
