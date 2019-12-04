/*
Copyright 2018 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	RequeueDuration1 = 50
	RequeueDuration2 = 40
)

var _ = Describe("Errors testing", func() {
	It("returns the correct error", func() {
		err := &RequeueAfterError{time.Second * RequeueDuration1}
		Expect(err.Error()).To(Equal(fmt.Sprintf("requeue in: %vs", RequeueDuration1)))
	})

	It("Gets the correct duration", func() {
		duration, _ := time.ParseDuration(fmt.Sprintf("%vs", RequeueDuration2))
		err := &RequeueAfterError{time.Second * RequeueDuration2}
		Expect(err.GetRequeueAfter()).To(Equal(duration))
	})
})
