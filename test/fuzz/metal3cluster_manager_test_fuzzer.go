package fuzz_test

import (
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ImageValidate struct {
	Image         infrav1.Image
	ErrorExpected bool
	Name          string
}

func FuzzTestImageValidate(data []byte) int {
	f := fuzz.NewConsumer(data)
	tc := &ImageValidate{}
	err := f.GenerateStruct(tc)
	if err != nil {
		return 0
	}
	errs := tc.Image.Validate(*field.NewPath("Spec", "Image"))
	if tc.ErrorExpected && errs == nil {
		return 0
	}
	if !tc.ErrorExpected && errs != nil {
		return 0
	}

	return 1
}
