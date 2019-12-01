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
	"testing"
	"time"
)

const (
	RequeueDuration1 = 50
	RequeueDuration2 = 40
)

func TestError(t *testing.T) {
	err := &RequeueAfterError{time.Second * RequeueDuration1}
	if err.Error() != fmt.Sprintf("requeue in: %vs", RequeueDuration1) {
		t.Errorf("Error, expected %vs, got %s", RequeueDuration1, err.RequeueAfter)
	}
}

func TestGetRequeueAfter(t *testing.T) {
	duration, _ := time.ParseDuration(fmt.Sprintf("%vs", RequeueDuration2))
	err := &RequeueAfterError{time.Second * RequeueDuration2}
	if err.GetRequeueAfter() != duration {
		t.Errorf("Error in duration, expected %vs, got %s", RequeueDuration2,
			err.RequeueAfter,
		)
	}
}
