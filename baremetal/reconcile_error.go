/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package baremetal

import (
	"fmt"
	"time"
)

// ReconcileError represents an generic error of Reconcile loop. ErrorType indicates what type
// of action is required to recover. It can take two values:
// 1. `Transient` - Can be recovered , will be requeued after.
// 2. `Terminal` - Cannot be recovered, will not be requeued.

type ReconcileError struct {
	error
	errorType    ReconcileErrorType
	RequeueAfter time.Duration
}

// ReconcileErrorType represents the type of a ReconcileError.
type ReconcileErrorType string

const (
	// TransientErrorType can be recovered, will be requeued after a configured time interval.
	TransientErrorType ReconcileErrorType = "Transient"
	// TerminalErrorType cannot be recovered, will not be requeued.
	TerminalErrorType ReconcileErrorType = "Terminal"
)

// Error returns the error message for a ReconcileError.
func (e ReconcileError) Error() string {
	var errStr string
	if e.error != nil {
		errStr = e.error.Error()
	}
	switch e.errorType {
	case TransientErrorType:
		return fmt.Sprintf("%s. Object will be requeued after %s", errStr, e.GetRequeueAfter())
	case TerminalErrorType:
		return fmt.Sprintf("reconcile error that cannot be recovered occurred: %s. Object will not be requeued", errStr)
	default:
		return "reconcile error occurred with unknown recovery type. The actual error is: " + errStr
	}
}

// GetRequeueAfter gets the duration to wait until the managed object is
// requeued for further processing.
func (e ReconcileError) GetRequeueAfter() time.Duration {
	return e.RequeueAfter
}

// IsTransient returns if the ReconcileError is recoverable.
func (e ReconcileError) IsTransient() bool {
	return e.errorType == TransientErrorType
}

// IsTerminal returns if the ReconcileError is non recoverable.
func (e ReconcileError) IsTerminal() bool {
	return e.errorType == TerminalErrorType
}

// WithTransientError wraps the error in a ReconcileError with errorType as `Transient`.
func WithTransientError(err error, requeueAfter time.Duration) ReconcileError {
	return ReconcileError{error: err, errorType: TransientErrorType, RequeueAfter: requeueAfter}
}

// WithTerminalError wraps the error in a ReconcileError with errorType as `Terminal`.
func WithTerminalError(err error) ReconcileError {
	return ReconcileError{error: err, errorType: TerminalErrorType}
}
