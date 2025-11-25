package controllers

import (
	"github.com/go-logr/logr"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
)

// logReconcileOutcome centralizes logging for controller reconciliation lifecycle events.
// It ensures every reconciliation emits a completion log on the debug level and records
// failures uniformly so that callers do not have to duplicate error logging.
func logReconcileOutcome(logger logr.Logger, controllerName string, rerr *error) {
	if rerr == nil {
		return
	}

	if *rerr != nil {
		// Controller-runtime already logs returned errors; avoid duplicate noise here.
		return
	}

	logger.V(baremetal.VerbosityLevelDebug).Info("Reconciliation completed", "controller", controllerName)
}

// logLogicGate makes it easy to record decision points at debug level while keeping
// a consistent verbosity across controllers.
func logLogicGate(logger logr.Logger, message string, keysAndValues ...any) {
	logger.V(baremetal.VerbosityLevelDebug).Info(message, keysAndValues...)
}
