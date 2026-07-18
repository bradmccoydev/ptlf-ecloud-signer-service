// Package metrics provides a dedicated HTTP server for Prometheus metrics exposition.
// The metrics server runs on a separate port (default 9090) from the main application
// server, following the platform pattern for observability endpoints.
package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewServer creates a new HTTP server configured to serve Prometheus metrics
// on the given port at the /metrics endpoint.
// The caller is responsible for starting and stopping the server.
func NewServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}
