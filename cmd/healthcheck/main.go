package main

import (
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func run(logger *logrus.Logger) int {
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:3000"
	}

	healthEndpoint := serverURL + "/health"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(healthEndpoint)
	if err != nil {
		logger.WithError(err).Error("Health check failed")
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
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
