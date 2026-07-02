//go:build !webui

package main

import (
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// registerWebUI does nothing when webui build tag is not set.
func registerWebUI(_ *httprouter.Router, logger *logrus.Logger) error {
	logger.Info("Web UI not embedded (build without -tags webui)")
	return nil
}
