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
	"strconv"

	"github.com/go-logr/logr"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DataTemplateManagerInterface is an interface for a DataTemplateManager.
type DataTemplateManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	SetClusterOwnerRef(*clusterv1.Cluster) error
	UpdateDatas(context.Context) (int, error)
}

// DataTemplateManager is responsible for performing machine reconciliation.
type DataTemplateManager struct {
	client       client.Client
	DataTemplate *capm3.Metal3DataTemplate
	Log          logr.Logger
}

// NewDataTemplateManager returns a new helper for managing a dataTemplate object.
func NewDataTemplateManager(client client.Client,
	dataTemplate *capm3.Metal3DataTemplate, dataTemplateLog logr.Logger) (*DataTemplateManager, error) {
	return &DataTemplateManager{
		client:       client,
		DataTemplate: dataTemplate,
		Log:          dataTemplateLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *DataTemplateManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.DataTemplate.Finalizers, capm3.DataTemplateFinalizer) {
		m.DataTemplate.Finalizers = append(m.DataTemplate.Finalizers,
			capm3.DataTemplateFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *DataTemplateManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.DataTemplate.Finalizers = Filter(m.DataTemplate.Finalizers,
		capm3.DataTemplateFinalizer,
	)
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
		if ok := errors.As(err, &notFoundErr); !ok {
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
	dataObjects := capm3.Metal3DataList{}
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

		if !m.dataObjectBelongsToTemplate(dataObject) {
			continue
		}

		// Get the claim Name, if unset use empty string, to still record the
		// index being used, to avoid conflicts
		claimName := ""
		if dataObject.Spec.Claim.Name != "" {
			claimName = dataObject.Spec.Claim.Name
		}
		m.DataTemplate.Status.Indexes[claimName] = dataObject.Spec.Index
		indexes[dataObject.Spec.Index] = claimName
	}
	m.updateStatusTimestamp()
	return indexes, nil
}

func (m *DataTemplateManager) dataObjectBelongsToTemplate(dataObject capm3.Metal3Data) bool {
	if dataObject.Spec.Template.Name == m.DataTemplate.Name {
		return true
	}

	// Match TemplateReference
	if dataObject.Spec.TemplateReference == "" && m.DataTemplate.Spec.TemplateReference == dataObject.Spec.Template.Name {
		return true
	}

	if dataObject.Spec.TemplateReference != "" && m.DataTemplate.Spec.TemplateReference == dataObject.Spec.TemplateReference {
		return true
	}
	return false
}

func (m *DataTemplateManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.DataTemplate.Status.LastUpdated = &now
}

// UpdateDatas manages the claims and creates or deletes Metal3Data accordingly.
// It returns the number of current allocations.
func (m *DataTemplateManager) UpdateDatas(ctx context.Context) (int, error) {
	indexes, err := m.getIndexes(ctx)
	if err != nil {
		return 0, err
	}

	// get list of Metal3DataClaim objects
	dataClaimObjects := capm3.Metal3DataClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.DataTemplate.Namespace,
	}

	err = m.client.List(ctx, &dataClaimObjects, opts)
	if err != nil {
		return 0, err
	}

	// Iterate over the Metal3Data objects to find all indexes and objects
	for _, dataClaim := range dataClaimObjects.Items {
		dataClaim := dataClaim
		// If DataTemplate does not point to this object, discard
		if dataClaim.Spec.Template.Name != m.DataTemplate.Name {
			continue
		}

		if dataClaim.Status.RenderedData != nil && dataClaim.DeletionTimestamp.IsZero() {
			continue
		}

		indexes, err = m.updateData(ctx, &dataClaim, indexes)
		if err != nil {
			return 0, err
		}
	}
	m.updateStatusTimestamp()
	return len(indexes), nil
}

func (m *DataTemplateManager) updateData(ctx context.Context,
	dataClaim *capm3.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	helper, err := patch.NewHelper(dataClaim, m.client)
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
	dataClaim *capm3.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	var dataName string

	if !Contains(dataClaim.Finalizers, capm3.DataClaimFinalizer) {
		dataClaim.Finalizers = append(dataClaim.Finalizers,
			capm3.DataClaimFinalizer,
		)
	}

	if dataClaimIndex, ok := m.DataTemplate.Status.Indexes[dataClaim.Name]; ok {
		if m.DataTemplate.Spec.TemplateReference != "" {
			dataName = m.DataTemplate.Spec.TemplateReference + "-" + strconv.Itoa(dataClaimIndex)
		} else {
			dataName = m.DataTemplate.Name + "-" + strconv.Itoa(dataClaimIndex)
		}

		dataClaim.Status.RenderedData = &corev1.ObjectReference{
			Name:      dataName,
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
		if ownerRef.Kind == "Metal3Machine" &&
			aGV.Group == capm3.GroupVersion.Group {
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
	for index := 0; index < len(indexes); index++ {
		if _, ok := indexes[index]; !ok {
			if claimIndex == len(indexes) {
				claimIndex = index
				break
			}
		}
	}

	// Set the index and Metal3Data names
	if m.DataTemplate.Spec.TemplateReference != "" {
		dataName = m.DataTemplate.Spec.TemplateReference + "-" + strconv.Itoa(claimIndex)
	} else {
		dataName = m.DataTemplate.Name + "-" + strconv.Itoa(claimIndex)
	}
	m.Log.Info("Index", "Claim", dataClaim.Name, "index", claimIndex)

	// Create the Metal3Data object, with an Owner ref to the Metal3Machine
	// (curOwnerRef) and to the Metal3DataTemplate
	dataObject := &capm3.Metal3Data{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3Data",
			APIVersion: capm3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataName,
			Namespace: m.DataTemplate.Namespace,
			Labels:    dataClaim.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					Controller: pointer.BoolPtr(true),
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
					Kind:       "Metal3Machine",
					Name:       m3mName,
					UID:        m3mUID,
				},
			},
		},
		Spec: capm3.Metal3DataSpec{
			Index:             claimIndex,
			TemplateReference: m.DataTemplate.Spec.TemplateReference,
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
	// HasRequeueAfterError), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(ctx, m.client, dataObject); err != nil {
		if ok := errors.As(err, &requeueAfterError); !ok {
			dataClaim.Status.ErrorMessage = pointer.StringPtr("Failed to create associated Metal3Data object")
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

// DeleteDatas deletes old secrets.
func (m *DataTemplateManager) deleteData(ctx context.Context,
	dataClaim *capm3.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	var dataName string
	m.Log.Info("Deleting Claim", "Metal3DataClaim", dataClaim.Name)

	dataClaimIndex, ok := m.DataTemplate.Status.Indexes[dataClaim.Name]
	if ok {
		// Try to get the Metal3Data. if it succeeds, delete it
		tmpM3Data := &capm3.Metal3Data{}

		if m.DataTemplate.Spec.TemplateReference != "" {
			dataName = m.DataTemplate.Spec.TemplateReference + "-" + strconv.Itoa(dataClaimIndex)
		} else {
			dataName = m.DataTemplate.Name + "-" + strconv.Itoa(dataClaimIndex)
		}

		key := client.ObjectKey{
			Name:      dataName,
			Namespace: m.DataTemplate.Namespace,
		}
		err := m.client.Get(ctx, key, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			dataClaim.Status.ErrorMessage = pointer.StringPtr("Failed to get associated Metal3Data object")
			return indexes, err
		} else if err == nil {
			// Delete the secret with metadata
			fmt.Println(tmpM3Data.Name)
			err = m.client.Delete(ctx, tmpM3Data)
			if err != nil && !apierrors.IsNotFound(err) {
				dataClaim.Status.ErrorMessage = pointer.StringPtr("Failed to delete associated Metal3Data object")
				return indexes, err
			}
		}
	}
	dataClaim.Status.RenderedData = nil
	dataClaim.Finalizers = Filter(dataClaim.Finalizers,
		capm3.DataClaimFinalizer,
	)

	m.Log.Info("Deleted Claim", "Metal3DataClaim", dataClaim.Name)

	if ok {
		delete(m.DataTemplate.Status.Indexes, dataClaim.Name)
		delete(indexes, dataClaimIndex)
	}
	m.updateStatusTimestamp()
	return indexes, nil
}
