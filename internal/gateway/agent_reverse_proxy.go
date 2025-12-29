package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go-x402-facilitator/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type AgentReverseProxy struct {
	proxy *httputil.ReverseProxy
}

// responseCapture is a custom ResponseWriter that captures the response
type responseCapture struct {
	http.ResponseWriter
	statusCode    int
	body          *bytes.Buffer
	headerWritten bool
	headers       http.Header
}

func NewResponseCapture(w http.ResponseWriter) *responseCapture {
	return &responseCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           bytes.NewBuffer(nil),
		headerWritten:  false,
		headers:        make(http.Header),
	}
}

func (rc *responseCapture) Header() http.Header {
	return rc.headers
}

func (rc *responseCapture) WriteHeader(code int) {
	if !rc.headerWritten {
		rc.statusCode = code
		rc.headerWritten = true
		// Don't write header yet, we'll write it after checking for 402
	}
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	if !rc.headerWritten {
		rc.WriteHeader(http.StatusOK)
	}
	rc.body.Write(b)
	// Don't write to original writer yet, we'll write it after checking for 402
	return len(b), nil
}

func (rc *responseCapture) flush() {
	// Copy headers to original ResponseWriter
	for key, values := range rc.headers {
		for _, value := range values {
			rc.ResponseWriter.Header().Add(key, value)
		}
	}
	if rc.headerWritten {
		rc.ResponseWriter.WriteHeader(rc.statusCode)
	}
	rc.ResponseWriter.Write(rc.body.Bytes())
}

func NewAgentReverseProxy(c *gin.Context, targetURL *url.URL) *AgentReverseProxy {
	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// req.URL.Path = c.Request.URL.Path
		req.URL.Path = targetURL.Path

		// Preserve original raw query
		req.URL.RawQuery = c.Request.URL.RawQuery

		log.Info().Msgf("Reverse Proxy Request URL: %s, %s, %s", req.URL.String(), req.URL.Path, req.URL.RawQuery)

		// Preserve other headers without X-Payment
		for key, values := range c.Request.Header {
			if key != "X-Payment" {
				req.Header[key] = values
			}
		}
	}

	// Handle errors
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Error().Err(err).Msg("Proxy error")
		rw.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(rw).Encode(types.ErrorResponse{
			Error:   "bad_gateway",
			Message: fmt.Sprintf("Failed to proxy request: %s", err.Error()),
			Code:    http.StatusBadGateway,
		})
	}

	return &AgentReverseProxy{proxy: proxy}
}

func (p *AgentReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}
