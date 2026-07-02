// Command healthcheck is a small CLI used by Docker's HEALTHCHECK
// instruction (and cmd/webapi's own /health route) to probe whether the
// WASAText server is up.
package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// httpClientTimeout bounds how long the health probe waits for a response.
const httpClientTimeout = 5 * time.Second

func run(logger *logrus.Logger) int {
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:3000"
	}

	healthEndpoint := serverURL + "/health"

	client := &http.Client{
		Timeout: httpClientTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpClientTimeout)
	defer cancel()

	// serverURL is operator-supplied deployment configuration (a Docker/K8s
	// env var naming this service's own address), not attacker-controlled
	// input, so the SSRF taint warning below doesn't apply.
	//nolint:gosec // G704: see comment above
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthEndpoint, nil)
	if err != nil {
		logger.WithError(err).Error("Health check failed")
		return 1
	}

	resp, err := client.Do(req) //nolint:gosec // G704: see comment above req's construction
	if err != nil {
		logger.WithError(err).Error("Health check failed")
		return 1
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Error("Failed to close response body")
		}
	}()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		logger.Info("Health check passed")
		return 0
	}

	logger.WithField("statusCode", resp.StatusCode).Error("Health check failed")
	return 1
}

func main() {
	logger := logrus.New()
	os.Exit(run(logger))
}
