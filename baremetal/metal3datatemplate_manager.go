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
	"strconv"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DataTemplateManagerInterface is an interface for a DataTemplateManager.
type DataTemplateManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	SetClusterOwnerRef(*clusterv1.Cluster) error
	// UpdateDatas handles the Metal3DataClaims and creates or deletes Metal3Data accordingly.
	// It returns if there are still Data object and undeleted DataClaims objects.
	UpdateDatas(context.Context) (bool, bool, error)
}

// DataTemplateManager is responsible for performing machine reconciliation.
type DataTemplateManager struct {
	client       client.Client
	DataTemplate *infrav1.Metal3DataTemplate
	Log          logr.Logger
}

// NewDataTemplateManager returns a new helper for managing a dataTemplate object.
func NewDataTemplateManager(client client.Client,
	dataTemplate *infrav1.Metal3DataTemplate, dataTemplateLog logr.Logger) (*DataTemplateManager, error) {
	return &DataTemplateManager{
		client:       client,
		DataTemplate: dataTemplate,
		Log:          dataTemplateLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *DataTemplateManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !controllerutil.ContainsFinalizer(m.DataTemplate, infrav1.DataTemplateFinalizer) {
		controllerutil.AddFinalizer(m.DataTemplate, infrav1.DataTemplateFinalizer)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *DataTemplateManager) UnsetFinalizer() {
	// Remove the finalizer.
	controllerutil.RemoveFinalizer(m.DataTemplate, infrav1.DataTemplateFinalizer)
}

// SetClusterOwnerRef sets ownerRef.
func (m *DataTemplateManager) SetClusterOwnerRef(cluster *clusterv1.Cluster) error {
	// Verify that the owner reference is there, if not add it and update object,
	// if error requeue.
	if cluster == nil {
		return errors.New("Missing cluster")
	}
	_, err := findOwnerRefFromList(m.DataTemplate.OwnerReferences,
		cluster.TypeMeta, cluster.ObjectMeta)
	if err != nil {
		if ok := errors.As(err, &errNotFound); !ok {
			return err
		}
		m.DataTemplate.OwnerReferences, err = setOwnerRefInList(
			m.DataTemplate.OwnerReferences, false, cluster.TypeMeta,
			cluster.ObjectMeta,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// RecreateStatus recreates the status if empty.
func (m *DataTemplateManager) getIndexes(ctx context.Context) (map[int]string, error) {
	m.Log.Info("Fetching Metal3Data objects")

	// start from empty maps
	m.DataTemplate.Status.Indexes = make(map[string]int)

	indexes := make(map[int]string)

	// get list of Metal3Data objects
	dataObjects := infrav1.Metal3DataList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.DataTemplate.Namespace,
	}

	err := m.client.List(ctx, &dataObjects, opts)
	if err != nil {
		return indexes, err
	}

	// Iterate over the Metal3Data objects to find all indexes and objects
	for _, dataObject := range dataObjects.Items {
		// If DataTemplate does not point to this object, discard
		if dataObject.Spec.Template.Name == "" {
			continue
		}
		if dataObject.Spec.Template.Name != m.DataTemplate.Name {
			continue
		}

		claimName := dataObject.Spec.Claim.Name
		m.DataTemplate.Status.Indexes[claimName] = dataObject.Spec.Index
		indexes[dataObject.Spec.Index] = claimName
	}
	m.updateStatusTimestamp()
	return indexes, nil
}

func (m *DataTemplateManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.DataTemplate.Status.LastUpdated = &now
}

// UpdateDatas handles the Metal3DataClaims and creates or deletes Metal3Data accordingly.
// It returns if there are still Data object and undeleted DataClaims objects.
func (m *DataTemplateManager) UpdateDatas(ctx context.Context) (bool, bool, error) {
	indexes, err := m.getIndexes(ctx)
	if err != nil {
		return false, false, err
	}
	hasData := len(indexes) > 0

	// get list of Metal3DataClaim objects
	dataClaimObjects := infrav1.Metal3DataClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.DataTemplate.Namespace,
		Limit:     DefaultListLimit,
	}

	err = m.client.List(ctx, &dataClaimObjects, opts)
	if err != nil {
		return false, false, err
	}

	hasClaims := false
	// Iterate over the Metal3Data objects to find all indexes and objects
	for _, dataClaim := range dataClaimObjects.Items {
		// If DataTemplate does not point to this object, discard
		if dataClaim.Spec.Template.Name != m.DataTemplate.Name {
			continue
		}
		if dataClaim.DeletionTimestamp.IsZero() {
			hasClaims = true
			if dataClaim.Status.RenderedData != nil {
				continue
			}
		}

		indexes, err = m.updateData(ctx, &dataClaim, indexes)
		if err != nil {
			return false, false, err
		}
	}
	m.updateStatusTimestamp()
	return hasData, hasClaims, nil
}

func (m *DataTemplateManager) updateData(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	helper, err := v1beta1patch.NewHelper(dataClaim, m.client)
	if err != nil {
		return indexes, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch dataClaim exiting this function so we can persist any changes.
	defer func() {
		err := helper.Patch(ctx, dataClaim)
		if err != nil {
			m.Log.Info("failed to Patch capm3DataClaim")
		}
	}()

	dataClaim.Status.ErrorMessage = nil

	if dataClaim.DeletionTimestamp.IsZero() {
		indexes, err = m.createData(ctx, dataClaim, indexes)
		if err != nil {
			return indexes, err
		}
	} else {
		indexes, err = m.deleteData(ctx, dataClaim, indexes)
		if err != nil {
			return indexes, err
		}
	}
	return indexes, nil
}

func (m *DataTemplateManager) createData(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	if !controllerutil.ContainsFinalizer(dataClaim, infrav1.DataClaimFinalizer) {
		controllerutil.AddFinalizer(dataClaim, infrav1.DataClaimFinalizer)
	}

	if dataClaimIndex, ok := m.DataTemplate.Status.Indexes[dataClaim.Name]; ok {
		dataClaim.Status.RenderedData = &corev1.ObjectReference{
			Name:      m.DataTemplate.Name + "-" + strconv.Itoa(dataClaimIndex),
			Namespace: m.DataTemplate.Namespace,
		}
		return indexes, nil
	}

	m3mUID := types.UID("")
	m3mName := ""
	for _, ownerRef := range dataClaim.OwnerReferences {
		aGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return indexes, err
		}
		if ownerRef.Kind == metal3MachineKind &&
			aGV.Group == infrav1.GroupVersion.Group {
			m3mUID = ownerRef.UID
			m3mName = ownerRef.Name
			break
		}
	}
	if m3mName == "" {
		return indexes, errors.New("Metal3Machine not found in owner references")
	}

	// Get a new index for this machine
	m.Log.Info("Getting index", "Claim", dataClaim.Name)
	claimIndex := len(indexes)
	// The length of the map might be smaller than the highest index stored,
	// this means we have a gap to find
	for index := range len(indexes) {
		if _, ok := indexes[index]; !ok {
			if claimIndex == len(indexes) {
				claimIndex = index
				break
			}
		}
	}

	// Set the index and Metal3Data names
	dataName := m.DataTemplate.Name + "-" + strconv.Itoa(claimIndex)

	m.Log.Info("Index", "Claim", dataClaim.Name, "index", claimIndex)

	// Create the Metal3Data object, with an Owner ref to the Metal3Machine
	// (curOwnerRef) and to the Metal3DataTemplate. Also add a finalizer.
	dataObject := &infrav1.Metal3Data{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3Data",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       dataName,
			Namespace:  m.DataTemplate.Namespace,
			Finalizers: []string{infrav1.DataClaimFinalizer},
			Labels:     dataClaim.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					Controller: ptr.To(true),
					APIVersion: m.DataTemplate.APIVersion,
					Kind:       m.DataTemplate.Kind,
					Name:       m.DataTemplate.Name,
					UID:        m.DataTemplate.UID,
				},
				{
					APIVersion: dataClaim.APIVersion,
					Kind:       dataClaim.Kind,
					Name:       dataClaim.Name,
					UID:        dataClaim.UID,
				},
				{
					APIVersion: dataClaim.APIVersion,
					Kind:       metal3MachineKind,
					Name:       m3mName,
					UID:        m3mUID,
				},
			},
		},
		Spec: infrav1.Metal3DataSpec{
			Index: claimIndex,
			Template: corev1.ObjectReference{
				Name:      m.DataTemplate.Name,
				Namespace: m.DataTemplate.Namespace,
			},
			Claim: corev1.ObjectReference{
				Name:      dataClaim.Name,
				Namespace: m.DataTemplate.Namespace,
			},
		},
	}

	// Create the Metal3Data object. If we get a conflict (that will set
	// TransientType ReconcileError), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(ctx, m.client, dataObject); err != nil {
		var reconcileError ReconcileError
		if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
			dataClaim.Status.ErrorMessage = ptr.To("Failed to create associated Metal3Data object")
		}
		return indexes, err
	}

	m.DataTemplate.Status.Indexes[dataClaim.Name] = claimIndex
	indexes[claimIndex] = dataClaim.Name

	dataClaim.Status.RenderedData = &corev1.ObjectReference{
		Name:      dataName,
		Namespace: m.DataTemplate.Namespace,
	}

	return indexes, nil
}

// deleteData deletes the Metal3DataClaim and marks the Metal3Data for deletion.
func (m *DataTemplateManager) deleteData(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	m.Log.Info("Deleting Metal3DataClaim", "Metal3DataClaim", dataClaim.Name)

	dataClaimIndex, ok := m.DataTemplate.Status.Indexes[dataClaim.Name]
	if ok {
		// Try to get the Metal3Data. if it succeeds, delete it
		tmpM3Data := &infrav1.Metal3Data{}
		key := client.ObjectKey{
			Name:      m.DataTemplate.Name + "-" + strconv.Itoa(dataClaimIndex),
			Namespace: m.DataTemplate.Namespace,
		}
		err := m.client.Get(ctx, key, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			dataClaim.Status.ErrorMessage = ptr.To("Failed to get associated Metal3Data object")
			return indexes, err
		} else if err == nil {
			// Remove the finalizer
			controllerutil.RemoveFinalizer(tmpM3Data, infrav1.DataClaimFinalizer)
			err = updateObject(ctx, m.client, tmpM3Data)
			if err != nil && !apierrors.IsNotFound(err) {
				m.Log.Info("Unable to remove finalizer from Metal3Data", "Metal3Data", tmpM3Data.Name)
				return indexes, err
			}
			// Delete the Metal3Data
			err = deleteObject(ctx, m.client, tmpM3Data)
			if err != nil && !apierrors.IsNotFound(err) {
				dataClaim.Status.ErrorMessage = ptr.To("Failed to delete associated Metal3Data object")
				return indexes, err
			}
			m.Log.Info("Deleted Metal3Data", "Metal3Data", tmpM3Data.Name)
		}
	}

	dataClaim.Status.RenderedData = nil
	controllerutil.RemoveFinalizer(dataClaim, infrav1.DataClaimFinalizer)

	if ok {
		delete(m.DataTemplate.Status.Indexes, dataClaim.Name)
		delete(indexes, dataClaimIndex)
	}

	m.Log.Info("Deleted Metal3DataClaim", "Metal3DataClaim", dataClaim.Name)
	m.updateStatusTimestamp()
	return indexes, nil
}
