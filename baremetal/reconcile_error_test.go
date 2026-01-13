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
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	duration = 50 * time.Second
)

var _ = Describe("Reconcile Error testing", func() {

	It("Returns correct values for Transient Error", func() {

		err := WithTransientError(errors.New("transient Error"), duration)
		Expect(err.GetRequeueAfter()).To(Equal(duration))
		Expect(err.IsTransient()).To(BeTrue())
		Expect(err.IsTerminal()).To(BeFalse())
		Expect(err.Error()).To(Equal(fmt.Sprintf("%s. Object will be requeued after %s", "transient Error", duration)))
	})

	It("Returns correct values for Terminal Error", func() {
		err := WithTerminalError(errors.New("terminal Error"))
		Expect(err.IsTransient()).To(BeFalse())
		Expect(err.IsTerminal()).To(BeTrue())
		Expect(err.Error()).To(Equal(fmt.Sprintf("reconcile error that cannot be recovered occurred: %s. Object will not be requeued", "terminal Error")))
	})

	It("Returns correct values for Unknown ReconcileError type", func() {
		err := ReconcileError{errors.New("unknown Error"), "unknownErrorType", 0 * time.Second}
		Expect(err.IsTerminal()).To(BeFalse())
		Expect(err.IsTransient()).To(BeFalse())
		Expect(err.Error()).To(Equal("reconcile error occurred with unknown recovery type. The actual error is: unknown Error"))
	})
})
