// Command webapi runs the WASAText HTTP API server.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourname/wasatext/service/api"
	"github.com/yourname/wasatext/service/db"
)

// HTTP server timeouts. Set explicitly rather than relying on
// http.ListenAndServe's defaults (none), which leave the server open to
// slow-client resource exhaustion.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	if err := execute(logger); err != nil {
		logger.WithError(err).Error("Application terminated with error")
		os.Exit(1)
	}
}

// prepareStorage ensures the photos directory and the SQLite database are
// ready to use before the server starts accepting requests, logging the
// exact path involved in each step so a misconfigured/unwritable volume
// fails fast with a specific, actionable error rather than surfacing later
// as a cryptic driver-level failure.
func prepareStorage(logger *logrus.Logger, dbPath string) (*db.SQLiteDatabase, error) {
	logger.WithField("photosDir", os.Getenv("PHOTOS_DIR")).Info("Checking photos directory")

	if err := api.EnsurePhotosDir(); err != nil {
		logger.WithError(err).Error("Failed to prepare photos directory")
		return nil, fmt.Errorf("failed to prepare photos directory: %w", err)
	}

	logger.WithField("dbPath", dbPath).Info("Initializing database")

	database, err := db.NewDatabase(dbPath)
	if err != nil {
		logger.WithError(err).Error("Failed to initialize database")
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return database, nil
}

func execute(logger *logrus.Logger) error {
	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = "3000"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./wasatext.db"
	}

	database, err := prepareStorage(logger, dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.WithError(closeErr).Error("Failed to close database")
		}
	}()

	logger.Info("Initializing API handler")

	apiHandler, err := api.NewHandler(api.Config{
		Database: database,
		Logger:   logger,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to initialize API handler")
		return fmt.Errorf("failed to initialize API handler: %w", err)
	}

	if err := registerWebUI(apiHandler.Router(), logger); err != nil {
		logger.WithError(err).Error("Failed to register web UI")
		return err
	}

	httpHandler := apiHandler.Handler()

	serverAddr := ":" + serverPort
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	logger.WithField("port", serverPort).Info("Starting WASAText server")

	if err := server.ListenAndServe(); err != nil {
		logger.WithError(err).Error("Server failed")
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}
