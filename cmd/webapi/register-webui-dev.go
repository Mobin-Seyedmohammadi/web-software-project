//go:build webui

package main

import (
	"io/fs"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/yourname/wasatext/webui"
)

// registerWebUI serves the embedded web UI.
func registerWebUI(router *httprouter.Router, logger *logrus.Logger) error {
	logger.Info("Registering embedded web UI")

	webUIFS, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		return err
	}

	fileServer := http.FileServer(http.FS(webUIFS))

	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file.
		fileServer.ServeHTTP(w, r)
	})

	logger.Info("Web UI registered successfully")
	return nil
}
