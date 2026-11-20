// Package metal3util provides shared helpers for the CAPM3 test module that are
// safe to import from both the e2e suite and the runtime extension handlers
// without pulling in heavy test-framework dependencies.
package metal3util

import (
	"context"
	"errors"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Metal3DataToMachineName returns the name of the Metal3Machine associated
// with the given Metal3Data, resolved via the owner reference on its
// Metal3DataClaim.
func Metal3DataToMachineName(ctx context.Context, cl client.Client, m3data infrav1.Metal3Data) (string, error) {
	if m3data.Spec.Claim == nil || m3data.Spec.Claim.Name == "" {
		return "", errors.New("Metal3Data missing spec.claim.name reference")
	}

	dataClaim := &infrav1.Metal3DataClaim{}
	claimKey := types.NamespacedName{Name: m3data.Spec.Claim.Name, Namespace: m3data.Namespace}
	if err := cl.Get(ctx, claimKey, dataClaim); err != nil {
		return "", err
	}

	for _, ownerRef := range dataClaim.OwnerReferences {
		gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			continue
		}
		if ownerRef.Kind == "Metal3Machine" && gv.Group == infrav1.GroupVersion.Group {
			return ownerRef.Name, nil
		}
	}

	return "", errors.New("Metal3Machine not found in Metal3DataClaim owner references")
}
