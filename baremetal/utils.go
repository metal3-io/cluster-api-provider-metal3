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
	"fmt"
	"slices"
	"strings"
	"time"

	// comment for go-lint.
	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// metal3SecretType defines the type of secret created by metal3.
	metal3SecretType corev1.SecretType = "infrastructure.cluster.x-k8s.io/secret"
	// metal3MachineKind is the Kind of the Metal3Machine.
	metal3MachineKind   = "Metal3Machine"
	VerbosityLevelDebug = 4
	VerbosityLevelTrace = 5
)

var (
	// ErrNoCluster is returned when the cluster
	// label could not be found on the object passed in.
	ErrNoCluster = fmt.Errorf("no %q label present", clusterv1beta1.ClusterNameLabel)
)

// Contains returns true if a list contains a string.
func Contains(list []string, strToSearch string) bool {
	return slices.Contains(list, strToSearch)
}

// NotFoundError represents that an object was not found.
type NotFoundError struct {
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return "Object not found"
}

func patchIfFound(ctx context.Context, helper *v1beta1patch.Helper, host client.Object) error {
	err := helper.Patch(ctx, host)
	if err != nil {
		notFound := true
		var aggr kerrors.Aggregate
		if ok := errors.As(err, &aggr); ok {
			for _, kerr := range aggr.Errors() {
				if !apierrors.IsNotFound(kerr) {
					notFound = false
				}
				if apierrors.IsConflict(kerr) {
					return WithTransientError(errors.New("Updating object failed"), 0*time.Second)
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

func updateObject(ctx context.Context, cl client.Client, obj client.Object) error {
	copiedObj, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return errors.New("Type assertion to client.Object failed")
	}
	err := cl.Update(ctx, copiedObj)
	if apierrors.IsConflict(err) {
		return WithTransientError(errors.New("Update object conflicts"), requeueAfter)
	}
	return err
}

func createObject(ctx context.Context, cl client.Client, obj client.Object) error {
	copiedObj, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return errors.New("Type assertion to client.Object failed")
	}
	err := cl.Create(ctx, copiedObj)
	if apierrors.IsAlreadyExists(err) {
		return WithTransientError(errors.New("Object already exists"), requeueAfter)
	}
	return err
}

func deleteObject(ctx context.Context, cl client.Client, obj client.Object) error {
	copiedObj, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return errors.New("Type assertion to client.Object failed")
	}
	err := cl.Delete(ctx, copiedObj)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func createSecret(ctx context.Context, cl client.Client, name string,
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
				clusterv1beta1.ClusterNameLabel: clusterName,
			},
			OwnerReferences: ownerRefs,
		},
		Data: content,
		Type: metal3SecretType,
	}

	secret, err := checkSecretExists(ctx, cl, name, namespace)
	if err == nil {
		// Update the secret with user data.
		secret.ObjectMeta.Labels = bootstrapSecret.ObjectMeta.Labels
		secret.ObjectMeta.OwnerReferences = bootstrapSecret.ObjectMeta.OwnerReferences
		bootstrapSecret.ObjectMeta = secret.ObjectMeta
		return updateObject(ctx, cl, bootstrapSecret)
	} else if apierrors.IsNotFound(err) {
		// Create the secret with user data.
		return createObject(ctx, cl, bootstrapSecret)
	}
	return err
}

func checkSecretExists(ctx context.Context, cl client.Client, name string,
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

func deleteSecret(ctx context.Context, cl client.Client, name string,
	namespace string,
) error {
	tmpBootstrapSecret, err := checkSecretExists(ctx, cl, name, namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err == nil {
		// unset the finalizers (remove all since we do not expect anything else
		// to control that object).
		tmpBootstrapSecret.Finalizers = []string{}
		err = updateObject(ctx, cl, &tmpBootstrapSecret)
		if err != nil {
			return err
		}
		// Delete the secret with metadata.
		err = cl.Delete(ctx, &tmpBootstrapSecret)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// fetchM3DataTemplate returns the Metal3DataTemplate object.
func fetchM3DataTemplate(ctx context.Context,
	templateRef *corev1.ObjectReference, cl client.Client, mLog logr.Logger,
	clusterName string,
) (*infrav1.Metal3DataTemplate, error) {
	// If the user did not specify a Metal3DataTemplate, just keep going.
	if templateRef == nil {
		return nil, nil //nolint:nilnil
	}
	if templateRef.Name == "" {
		return nil, errors.New("Metal3DataTemplate name not set")
	}

	// Fetch the Metal3DataTemplate.
	metal3DataTemplate := &infrav1.Metal3DataTemplate{}
	metal3DataTemplateName := types.NamespacedName{
		Namespace: templateRef.Namespace,
		Name:      templateRef.Name,
	}
	if err := cl.Get(ctx, metal3DataTemplateName, metal3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			errMessage := "Metal3DataTemplate is not found, requeuing"
			mLog.Info(errMessage)
			return nil, WithTransientError(errors.New(errMessage), requeueAfter)
		}
		err = errors.Wrap(err, "Failed to get Metal3DataTemplate")
		return nil, err
	}

	// Verify that this Metal3DataTemplate belongs to the correct cluster.
	if clusterName != metal3DataTemplate.Spec.ClusterName {
		return nil, errors.New("Metal3DataTemplate associated with another cluster")
	}

	return metal3DataTemplate, nil
}

// fetchM3DataClaim returns the Metal3DataClaim object.
func fetchM3DataClaim(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string,
) (*infrav1.Metal3DataClaim, error) {
	// Fetch the Metal3DataClaim.
	m3DataClaim := &infrav1.Metal3DataClaim{}
	metal3DataClaimName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := cl.Get(ctx, metal3DataClaimName, m3DataClaim); err != nil {
		if apierrors.IsNotFound(err) {
			errMessage := "Metal3DataClaim is not found, requeuing"
			mLog.Info(errMessage)
			return nil, WithTransientError(errors.New(errMessage), requeueAfter)
		}
		err = errors.Wrap(err, "Failed to get Metal3DataClaim")
		return nil, err
	}
	return m3DataClaim, nil
}

// fetchM3Data returns the Metal3Data object.
func fetchM3Data(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string,
) (*infrav1.Metal3Data, error) {
	// Fetch the Metal3Data.
	m3Data := &infrav1.Metal3Data{}
	metal3DataName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := cl.Get(ctx, metal3DataName, m3Data); err != nil {
		if apierrors.IsNotFound(err) {
			errMessage := "Metal3Data is not found, requeuing"
			mLog.Info(errMessage)
			return nil, WithTransientError(errors.New(errMessage), requeueAfter)
		}
		err = errors.Wrap(err, "Failed to get Metal3Data")
		return nil, err
	}
	return m3Data, nil
}

// getM3Machine returns the Metal3Machine object.
func getM3Machine(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string, dataTemplate *infrav1.Metal3DataTemplate,
	requeueifNotFound bool,
) (*infrav1.Metal3Machine, error) {
	// Get the Metal3Machine.
	tmpM3Machine := &infrav1.Metal3Machine{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	err := cl.Get(ctx, key, tmpM3Machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			mLog.Info("Metal3Machine is not found")
			if requeueifNotFound {
				errMessage := "Metal3Machine is not found, requeuing"
				mLog.Info(errMessage)
				return nil, WithTransientError(errors.New(errMessage), requeueAfter)
			}
			return nil, nil //nolint:nilnil
		}
		err = errors.Wrap(err, "Failed to get Metal3Machine")
		return nil, err
	}

	if dataTemplate == nil {
		return tmpM3Machine, nil
	}

	// Verify that the Metal3Machine fulfills the conditions.
	if tmpM3Machine.Spec.DataTemplate == nil {
		return nil, nil //nolint:nilnil
	}
	if tmpM3Machine.Spec.DataTemplate.Name != dataTemplate.Name {
		return nil, nil //nolint:nilnil
	}
	if tmpM3Machine.Spec.DataTemplate.Namespace != "" &&
		tmpM3Machine.Spec.DataTemplate.Namespace != dataTemplate.Namespace {
		return nil, nil //nolint:nilnil
	}
	return tmpM3Machine, nil
}

func parseProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderIDPrefix)
}

func ConvertTypedLocalObjectReferenceToIPPoolReference(
	ref corev1.TypedLocalObjectReference,
) capipamv1.IPPoolReference {
	if ref.APIGroup == nil || *ref.APIGroup == "" {
		ref.APIGroup = ptr.To("ipam.metal3.io")
	}
	if ref.Kind == "" {
		ref.Kind = "IPPool"
	}
	return capipamv1.IPPoolReference{
		Name:     ref.Name,
		APIGroup: *ref.APIGroup,
		Kind:     ref.Kind,
	}
}

func ConvertIPPoolReferenceToTypedLocalObjectReference(
	ref capipamv1.IPPoolReference,
) corev1.TypedLocalObjectReference {
	if ref.APIGroup == "" {
		ref.APIGroup = "ipam.metal3.io"
	}
	if ref.Kind == "" {
		ref.Kind = "IPPool"
	}
	return corev1.TypedLocalObjectReference{
		Name:     ref.Name,
		APIGroup: &ref.APIGroup,
		Kind:     ref.Kind,
	}
}
