/*
Copyright 2021 The Kubernetes Authors.

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
	"context"
	"strings"

	// comment for go-lint
	"github.com/go-logr/logr"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// metal3SecretType defines the type of secret created by metal3
	metal3SecretType corev1.SecretType = "infrastructure.cluster.x-k8s.io/secret"
)

// Filter filters a list for a string.
func Filter(list []string, strToFilter string) (newList []string) {
	for _, item := range list {
		if item != strToFilter {
			newList = append(newList, item)
		}
	}
	return
}

// Contains returns true if a list contains a string.
func Contains(list []string, strToSearch string) bool {
	for _, item := range list {
		if item == strToSearch {
			return true
		}
	}
	return false
}

// NotFoundError represents that an object was not found
type NotFoundError struct {
}

// Error implements the error interface
func (e *NotFoundError) Error() string {
	return "Object not found"
}

func patchIfFound(ctx context.Context, helper *patch.Helper, host client.Object) error {
	err := helper.Patch(ctx, host)
	if err != nil {
		notFound := true
		if aggr, ok := err.(kerrors.Aggregate); ok {
			for _, kerr := range aggr.Errors() {
				if !apierrors.IsNotFound(kerr) {
					notFound = false
				}
				if apierrors.IsConflict(kerr) {
					return &RequeueAfterError{}
				}
			}
		} else {
			notFound = false
		}
		if notFound {
			return nil
		}
	}
	return err
}

func updateObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	err := cl.Update(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsConflict(err) {
		return &RequeueAfterError{}
	}
	return err
}

func createObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	err := cl.Create(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsAlreadyExists(err) {
		return &RequeueAfterError{}
	}
	return err
}

func deleteObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	err := cl.Delete(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func createSecret(cl client.Client, ctx context.Context, name string,
	namespace string, clusterName string,
	ownerRefs []metav1.OwnerReference, content map[string][]byte,
) error {
	bootstrapSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				capi.ClusterLabelName: clusterName,
			},
			OwnerReferences: ownerRefs,
		},
		Data: content,
		Type: metal3SecretType,
	}

	secret, err := checkSecretExists(cl, ctx, name, namespace)
	if err == nil {
		// Update the secret with user data
		secret.ObjectMeta.Labels = bootstrapSecret.ObjectMeta.Labels
		secret.ObjectMeta.OwnerReferences = bootstrapSecret.ObjectMeta.OwnerReferences
		bootstrapSecret.ObjectMeta = secret.ObjectMeta
		return updateObject(cl, ctx, bootstrapSecret)
	} else if apierrors.IsNotFound(err) {
		// Create the secret with user data
		return createObject(cl, ctx, bootstrapSecret)
	}
	return err
}

func checkSecretExists(cl client.Client, ctx context.Context, name string,
	namespace string,
) (corev1.Secret, error) {
	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	err := cl.Get(ctx, key, &tmpBootstrapSecret)
	return tmpBootstrapSecret, err
}

func deleteSecret(cl client.Client, ctx context.Context, name string,
	namespace string,
) error {
	tmpBootstrapSecret, err := checkSecretExists(cl, ctx, name, namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err == nil {
		//unset the finalizers (remove all since we do not expect anything else
		// to control that object)
		tmpBootstrapSecret.Finalizers = []string{}
		err = updateObject(cl, ctx, &tmpBootstrapSecret)
		if err != nil {
			return err
		}
		// Delete the secret with metadata
		err = cl.Delete(ctx, &tmpBootstrapSecret)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// fetchMetadata fetches the Metal3DataTemplate object
func fetchM3DataTemplate(ctx context.Context,
	templateRef *corev1.ObjectReference, cl client.Client, mLog logr.Logger,
	clusterName string,
) (*capm3.Metal3DataTemplate, error) {

	// If the user did not specify a DataTemplate, just keep going
	if templateRef == nil {
		return nil, nil
	}
	if templateRef.Name == "" {
		return nil, errors.New("Metadata name not set")
	}

	// Fetch the Metal3 metadata.
	metal3DataTemplate := &capm3.Metal3DataTemplate{}
	metal3DataTemplateName := types.NamespacedName{
		Namespace: templateRef.Namespace,
		Name:      templateRef.Name,
	}
	if err := cl.Get(ctx, metal3DataTemplateName, metal3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			mLog.Info("Metadata not found, requeuing")
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			err := errors.Wrap(err, "Failed to get metadata")
			return nil, err
		}
	}

	// Verify that this Metal3Data belongs to the correct cluster
	if clusterName != metal3DataTemplate.Spec.ClusterName {
		return nil, errors.New("Metal3DataTemplate associated with another cluster")
	}

	return metal3DataTemplate, nil
}

func fetchM3DataClaim(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string,
) (*capm3.Metal3DataClaim, error) {
	// Fetch the Metal3Data
	m3Data := &capm3.Metal3DataClaim{}
	metal3DataName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := cl.Get(ctx, metal3DataName, m3Data); err != nil {
		if apierrors.IsNotFound(err) {
			mLog.Info("Data Claim not found")
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			err := errors.Wrap(err, "Failed to get metadata")
			return nil, err
		}
	}
	return m3Data, nil
}

func fetchM3Data(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string,
) (*capm3.Metal3Data, error) {
	// Fetch the Metal3Data
	m3Data := &capm3.Metal3Data{}
	metal3DataName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := cl.Get(ctx, metal3DataName, m3Data); err != nil {
		if apierrors.IsNotFound(err) {
			mLog.Info("Rendered data not found, requeuing")
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			err := errors.Wrap(err, "Failed to get metadata")
			return nil, err
		}
	}
	return m3Data, nil
}

func getM3Machine(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string, dataTemplate *capm3.Metal3DataTemplate,
	requeueifNotFound bool,
) (*capm3.Metal3Machine, error) {

	// Get the Metal3Machine
	tmpM3Machine := &capm3.Metal3Machine{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	err := cl.Get(ctx, key, tmpM3Machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if requeueifNotFound {
				return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
			}
			return nil, nil
		} else {
			return nil, err
		}
	}

	if dataTemplate == nil {
		return tmpM3Machine, nil
	}

	// Verify that the Metal3Machine fulfills the conditions
	if tmpM3Machine.Spec.DataTemplate == nil {
		return nil, nil
	}
	if tmpM3Machine.Spec.DataTemplate.Name != dataTemplate.Name {
		return nil, nil
	}
	if tmpM3Machine.Spec.DataTemplate.Namespace != "" &&
		tmpM3Machine.Spec.DataTemplate.Namespace != dataTemplate.Namespace {
		return nil, nil
	}
	return tmpM3Machine, nil
}

func parseProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderIDPrefix)
}
