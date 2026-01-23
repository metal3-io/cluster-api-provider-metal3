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
	"errors"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
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
		m.Log.V(VerbosityLevelTrace).Info("Adding finalizer to Metal3DataTemplate")
		controllerutil.AddFinalizer(m.DataTemplate, infrav1.DataTemplateFinalizer)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *DataTemplateManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.Log.V(VerbosityLevelTrace).Info("Removing finalizer from Metal3DataTemplate")
	controllerutil.RemoveFinalizer(m.DataTemplate, infrav1.DataTemplateFinalizer)
}

// SetClusterOwnerRef sets ownerRef.
func (m *DataTemplateManager) SetClusterOwnerRef(cluster *clusterv1.Cluster) error {
	m.Log.V(VerbosityLevelTrace).Info("Setting cluster owner reference on Metal3DataTemplate")
	// Verify that the owner reference is there, if not add it and update object,
	// if error requeue.
	if cluster == nil {
		return errors.New("missing cluster")
	}
	_, err := findOwnerRefFromList(m.DataTemplate.OwnerReferences,
		cluster.TypeMeta, cluster.ObjectMeta)
	if err != nil {
		if ok := errors.As(err, &errNotFound); !ok {
			return err
		}
		m.Log.V(VerbosityLevelDebug).Info("Adding cluster owner reference",
			LogFieldMetal3DataTemplate, m.DataTemplate.Name,
			LogFieldCluster, cluster.Name)
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
	m.Log.V(VerbosityLevelTrace).Info("Fetching Metal3Data objects for indexing",
		LogFieldMetal3DataTemplate, m.DataTemplate.Name)

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
	m.Log.V(VerbosityLevelTrace).Info("Updating Metal3Datas for template",
		LogFieldMetal3DataTemplate, m.DataTemplate.Name)
	indexes, err := m.getIndexes(ctx)
	if err != nil {
		return false, false, err
	}
	hasData := len(indexes) > 0
	m.Log.V(VerbosityLevelDebug).Info("Found existing Metal3Data indexes",
		LogFieldMetal3DataTemplate, m.DataTemplate.Name,
		LogFieldCount, len(indexes))

	// get list of Metal3DataClaim objects
	dataClaimObjects := infrav1.Metal3DataClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.DataTemplate.Namespace,
	}

	err = m.client.List(ctx, &dataClaimObjects, opts)
	if err != nil {
		return false, false, err
	}

	hasClaims := false
	// Iterate over the Metal3DataClaim objects to find all indexes and objects
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
		m.Log.V(VerbosityLevelTrace).Info("Initiate updating data of claim", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name)
		indexes, err = m.updateData(ctx, &dataClaim, indexes)
		if err != nil {
			return false, false, err
		}
		m.Log.V(VerbosityLevelDebug).Info("Success updating data of claim", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name)
	}
	m.updateStatusTimestamp()
	return hasData, hasClaims, nil
}

func (m *DataTemplateManager) updateData(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	m.Log.V(VerbosityLevelTrace).Info("Updating data for claim",
		LogFieldMetal3DataClaim, dataClaim.Name)
	helper, err := v1beta1patch.NewHelper(dataClaim, m.client)
	if err != nil {
		return indexes, fmt.Errorf("failed to init patch helper: %w", err)
	}
	// Always patch dataClaim exiting this function so we can persist any changes.
	defer func() {
		err = helper.Patch(ctx, dataClaim)
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
		m.Log.V(VerbosityLevelTrace).Info("Attempting to delete data of claim and related data if present", LogFieldMetal3DataClaim, dataClaim.Name)
		indexes, err = m.deleteMetal3DataAndClaim(ctx, dataClaim, indexes)
		if err != nil {
			return indexes, err
		}
	}
	return indexes, nil
}

func (m *DataTemplateManager) createData(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	m.Log.V(VerbosityLevelTrace).Info("Creating data for claim",
		LogFieldMetal3DataClaim, dataClaim.Name)
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
		return indexes, errors.New("metal3Machine not found in owner references")
	}

	// Get a new index for this machine
	m.Log.Info("Getting index", LogFieldMetal3DataClaim, dataClaim.Name)
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

	m.Log.Info("Index assigned", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldIndex, claimIndex)

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

// handleRetrieveDataError handles errors generated by Metal3Data retrieval.
// First return value is true if Metal3Data was retrieved successfully.
// Second return value contains an error message if an unknown error was encountered.
func (m *DataTemplateManager) handleRetrieveDataError(err error, filter string, dataClaimName string, dataName string) (bool, string) {
	if err != nil && apierrors.IsNotFound(err) {
		m.Log.Error(err, "Metal3Data NOT FOUND", "filter", filter, LogFieldMetal3DataClaim, dataClaimName, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
	} else if err != nil && !apierrors.IsNotFound(err) {
		persistentErrMsg := "Failed to get Metal3Data object for reason OTHER THAN not finding it based on " + filter
		m.Log.Error(err, persistentErrMsg, LogFieldMetal3DataClaim, dataClaimName, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
		return false, persistentErrMsg
	} else if err == nil {
		m.Log.V(VerbosityLevelTrace).Info("Metal3Data found!", LogFieldMetal3DataClaim, dataClaimName, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
		return true, ""
	}
	return false, ""
}

// retrieveData is a utility to retrieve Metal3Data based on name and save the data to specified object.
func (m *DataTemplateManager) retrieveData(ctx context.Context, dataName string, tmpM3Data *infrav1.Metal3Data) error {
	key := client.ObjectKey{
		Name:      dataName,
		Namespace: m.DataTemplate.Namespace,
	}
	return m.client.Get(ctx, key, tmpM3Data)
}

// deleteMetal3DataAndClaim deletes the Metal3DataClaim and marks the Metal3Data for deletion.
func (m *DataTemplateManager) deleteMetal3DataAndClaim(ctx context.Context,
	dataClaim *infrav1.Metal3DataClaim, indexes map[int]string,
) (map[int]string, error) {
	m.Log.Info("Deleting Metal3DataClaim", LogFieldMetal3DataClaim, dataClaim.Name)
	persistentErrMsg := ""
	m3DataFound := false
	tmpM3Data := &infrav1.Metal3Data{}
	dataName := ""

	dataClaimIndex, ok := m.DataTemplate.Status.Indexes[dataClaim.Name]

	if ok {
		// Try to get the Metal3Data, if it succeeds, delete it
		dataName = m.DataTemplate.Name + "-" + strconv.Itoa(dataClaimIndex)
		err := m.retrieveData(ctx, dataName, tmpM3Data)
		m3DataFound, persistentErrMsg = m.handleRetrieveDataError(err, "template name and claim index", dataClaim.Name, dataName)
	} else {
		persistentErrMsg = "index of the claim was not found"
		m.Log.Error(errors.New(persistentErrMsg), "Claim index not found", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name)
	}

	if !m3DataFound {
		m.Log.V(VerbosityLevelTrace).Info("Attempting to retrieve Metal3Data based on Metal3DataClaim render information")
		if dataClaim != nil && dataClaim.Status.RenderedData != nil {
			dataName = dataClaim.Status.RenderedData.Name
			err := m.retrieveData(ctx, dataName, tmpM3Data)
			errMsg := ""
			m3DataFound, errMsg = m.handleRetrieveDataError(err, "render data reference", dataClaim.Name, dataName)
			persistentErrMsg += errMsg
		}
	}

	if m3DataFound {
		// Remove the finalizer
		m.Log.V(VerbosityLevelTrace).Info("Attempting to remove finalizer from associated Metal3Data", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
		controllerutil.RemoveFinalizer(tmpM3Data, infrav1.DataClaimFinalizer)
		err := updateObject(ctx, m.client, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			m.Log.Error(errors.New("unable to remove finalizer from Metal3Data"), "Finalizer removal failed", LogFieldMetal3Data, tmpM3Data.Name)
			return indexes, err
		}
		// Delete the Metal3Data
		m.Log.V(VerbosityLevelDebug).Info("Deleting associated Metal3Data", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
		err = deleteObject(ctx, m.client, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			dataClaim.Status.ErrorMessage = ptr.To("Failed to delete associated Metal3Data object")
			return indexes, err
		}
		m.Log.Info("Deleted Metal3Data", LogFieldMetal3Data, tmpM3Data.Name)
	} else {
		errMsg := "failed to retrieve Metal3Data object because it was not found or for other unknown reason"
		persistentErrMsg += errMsg
		dataClaim.Status.ErrorMessage = ptr.To(persistentErrMsg)
		m.Log.Error(errors.New(errMsg), "error added to Metal3DataClaim status", LogFieldMetal3DataClaim, dataClaim.Name, LogFieldMetal3DataTemplate, m.DataTemplate.Name, LogFieldMetal3Data, dataName)
	}

	dataClaim.Status.RenderedData = nil
	controllerutil.RemoveFinalizer(dataClaim, infrav1.DataClaimFinalizer)

	if ok {
		delete(m.DataTemplate.Status.Indexes, dataClaim.Name)
		delete(indexes, dataClaimIndex)
	}

	m.Log.Info("Deleted Metal3DataClaim", LogFieldMetal3DataClaim, dataClaim.Name)
	m.updateStatusTimestamp()
	return indexes, nil
}
