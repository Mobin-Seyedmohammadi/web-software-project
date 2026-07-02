package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
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
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("Health check passed")
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Health check failed with status code: %d\n", resp.StatusCode)
	os.Exit(1)
}
