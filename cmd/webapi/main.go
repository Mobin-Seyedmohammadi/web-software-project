package main

import (
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/yourname/wasatext/service/api"
	"github.com/yourname/wasatext/service/db"
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
		logger.WithError(err).Fatal("Application terminated with error")
	}
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

	logger.WithField("dbPath", dbPath).Info("Initializing database")
	database, err := db.NewDatabase(dbPath)
	if err != nil {
		logger.WithError(err).Error("Failed to initialize database")
		return err
	}
	defer database.Close()

	logger.Info("Initializing API handler")
	apiHandler, err := api.NewHandler(api.Config{
		Database: database,
		Logger:   logger,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to initialize API handler")
		return err
	}

	if err := registerWebUI(apiHandler.Router(), logger); err != nil {
		logger.WithError(err).Error("Failed to register web UI")
		return err
	}

	httpHandler := apiHandler.Handler()

	serverAddr := ":" + serverPort
	logger.WithField("port", serverPort).Info("Starting WASAText server")

	if err := http.ListenAndServe(serverAddr, httpHandler); err != nil {
		logger.WithError(err).Error("Server failed")
		return err
	}

	return nil
}
