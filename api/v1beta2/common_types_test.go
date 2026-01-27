/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestImageValidate(t *testing.T) {
	diskFormat := LiveISODiskFormat
	cases := []struct {
		Image         Image
		ErrorExpected bool
		Name          string
	}{
		{
			Image:         Image{},
			ErrorExpected: true,
			Name:          "empty spec",
		},
		{
			Image: Image{
				URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
				Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
			},
			ErrorExpected: false,
			Name:          "Valid Image with Image.Checksum as URL",
		},
		{
			Image: Image{
				URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
				Checksum: "f7600f7a274d974a236c4da5161265859c32da93a7c8de6a77d560378a1384ef",
			},
			ErrorExpected: false,
			Name:          "Valid Image with Image.Checksum as sha256 sum",
		},
		{
			Image: Image{
				Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
			},
			ErrorExpected: true,
			Name:          "missing Image.URL",
		},
		{
			Image: Image{
				URL: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
			},
			ErrorExpected: true,
			Name:          "missing Image.Checksum",
		},
		{
			Image: Image{
				URL:      "test url",
				Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.sha256sum",
			},
			ErrorExpected: true,
			Name:          "Invalid URL Image.URL",
		},
		{
			Image: Image{
				URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
				Checksum: "http://:invalid",
			},
			ErrorExpected: true,
			Name:          "Invalid Image.Checksum http url",
		},
		{
			Image: Image{
				URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
				Checksum: "https://:invalid",
			},
			ErrorExpected: true,
			Name:          "Invalid Image.Checksum https url",
		},
		{
			Image: Image{
				URL:        "http://172.22.0.1/images/rhcos.iso",
				DiskFormat: &diskFormat,
			},
			ErrorExpected: false,
			Name:          "Valid spec with live-iso diskFormat",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			g := NewWithT(t)
			errs := tc.Image.Validate(*field.NewPath("Spec", "Image"))
			if tc.ErrorExpected {
				g.Expect(errs).ToNot(BeEmpty())
			} else {
				g.Expect(errs).To(BeEmpty())
			}
		})
	}
}
