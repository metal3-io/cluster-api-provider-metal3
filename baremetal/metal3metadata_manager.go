/*
Copyright 2020 The Kubernetes Authors.

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
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"text/template"

	// comment for go-lint
	"github.com/go-logr/logr"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	metaDataFinalizer = "metal3metadata.infrastructure.cluster.x-k8s.io/metaData"
)

// MetadataManagerInterface is an interface for a MetadataManager
type MetadataManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	RecreateStatus(context.Context) error
	DeleteSecrets(context.Context) error
	CreateSecrets(context.Context) error
	DeleteReady() (bool, error)
}

// MetadataManager is responsible for performing machine reconciliation
type MetadataManager struct {
	client   client.Client
	Metadata *capm3.Metal3Metadata
	Log      logr.Logger
}

// NewMetadataManager returns a new helper for managing a metadata object
func NewMetadataManager(client client.Client,
	metadata *capm3.Metal3Metadata, metadataLog logr.Logger) (*MetadataManager, error) {

	return &MetadataManager{
		client:   client,
		Metadata: metadata,
		Log:      metadataLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *MetadataManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.Metadata.Finalizers, capm3.MetadataFinalizer) {
		m.Metadata.Finalizers = append(m.Metadata.Finalizers,
			capm3.MetadataFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *MetadataManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	m.Metadata.Finalizers = Filter(m.Metadata.Finalizers,
		capm3.MetadataFinalizer,
	)
}

// RecreateStatus recreates the status if empty
func (m *MetadataManager) RecreateStatus(ctx context.Context) error {

	if m.Metadata.Status.LastUpdated != nil {
		return nil
	}

	if m.Metadata.Status.Indexes != nil && m.Metadata.Status.Secrets != nil {
		return nil
	}
	m.Log.Info("Recreating the Metal3Metadata status")
	m.Metadata.Status.Indexes = make(map[string]string)
	m.Metadata.Status.Secrets = make(map[string]corev1.SecretReference)

	for _, curOwnerRef := range m.Metadata.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return err
		}
		if curOwnerRef.Kind != "Metal3Machine" ||
			curOwnerRefGV.Group != capm3.GroupVersion.Group {
			continue
		}

		m.Log.Info("Verifying the owner", "Metal3machine", curOwnerRef.Name)
		// Verify that we have an owner ref machine that points to this Metadata
		m3Machine, err := m.getM3Machine(m.client, ctx, curOwnerRef)
		if err != nil {
			return err
		}
		if m3Machine == nil {
			continue
		}

		if m3Machine.Spec.MetaData.DataSecret == nil {
			continue
		}
		re := regexp.MustCompile(`\d+$`)
		machineIndex := re.FindString(m3Machine.Spec.MetaData.DataSecret.Name)
		if machineIndex == "" {
			continue
		}
		m.Metadata.Status.Indexes[machineIndex] = curOwnerRef.Name
		m.Log.Info("Index added", "Metal3machine", curOwnerRef.Name)

		secretNamespace := m.Metadata.Namespace
		if m3Machine.Spec.MetaData.DataSecret.Namespace != "" {
			secretNamespace = m3Machine.Spec.MetaData.DataSecret.Namespace
		}
		tmpBootstrapSecret := corev1.Secret{}
		key := client.ObjectKey{
			Name:      m3Machine.Spec.MetaData.DataSecret.Namespace,
			Namespace: secretNamespace,
		}
		err = m.client.Get(ctx, key, &tmpBootstrapSecret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				m.Log.Info("No metadata secret found", "Metal3machine", curOwnerRef.Name)
				continue
			}
			return err
		}
		m.Metadata.Status.Secrets[curOwnerRef.Name] = *m3Machine.Spec.MetaData.DataSecret
		m.Log.Info("Secret added", "Metal3machine", curOwnerRef.Name)
	}
	m.updateStatusTimestamp()
	return nil
}

func (m *MetadataManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.Metadata.Status.LastUpdated = &now
}

// CreateSecrets creates the missing secrets
func (m *MetadataManager) CreateSecrets(ctx context.Context) error {
	pendingItems := make(map[string]*capm3.Metal3Machine)
	needsUpdate := false
	var err error

	for _, curOwnerRef := range m.Metadata.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return err
		}
		if curOwnerRef.Kind != "Metal3Machine" ||
			curOwnerRefGV.Group != capm3.GroupVersion.Group {
			continue
		}
		dataSecret, ok := m.Metadata.Status.Secrets[curOwnerRef.Name]
		if ok && dataSecret.Name != "" {
			continue
		}

		m.Log.Info("Verifying the owner", "Metal3machine", curOwnerRef.Name)

		// Verify that we have an owner ref machine that points to this Metadata
		m3Machine, err := m.getM3Machine(m.client, ctx, curOwnerRef)
		if err != nil {
			return err
		}
		if m3Machine == nil {
			continue
		}

		m.Log.Info("Getting index", "Metal3machine", curOwnerRef.Name)
		machineIndex := ""
		for key, value := range m.Metadata.Status.Indexes {
			if value == curOwnerRef.Name {
				machineIndex = key
				break
			}
		}

		if machineIndex == "" {
			machineIndexInt := len(m.Metadata.Status.Indexes)
			// The length of the map might be smaller than the highest index stored,
			// this means we have a gap to find
			for index := 0; index < len(m.Metadata.Status.Indexes); index++ {
				if _, ok := m.Metadata.Status.Indexes[strconv.Itoa(index)]; !ok {
					if machineIndexInt == len(m.Metadata.Status.Indexes) {
						machineIndexInt = index
						break
					}
				}
			}
			machineIndex = strconv.Itoa(machineIndexInt)

			if m.Metadata.Status.Indexes == nil {
				m.Metadata.Status.Indexes = make(map[string]string)
			}
			m.Metadata.Status.Indexes[machineIndex] = curOwnerRef.Name
			m.Log.Info("Index", "Metal3machine", curOwnerRef.Name, "index", machineIndex)
			needsUpdate = true
		}

		pendingItems[machineIndex] = m3Machine
	}
	if needsUpdate {
		err = updateObject(m.client, ctx, m.Metadata)
		if err != nil {
			return err
		}
	}

	if m.Metadata.Status.Secrets == nil {
		m.Metadata.Status.Secrets = make(map[string]corev1.SecretReference)
	}

	for machineIndex, m3Machine := range pendingItems {
		m.Log.Info("Creating secret", "Metal3machine", m3Machine.Name)
		tmpl, err := createTemplate(m3Machine, machineIndex)
		if err != nil {
			return err
		}
		metadataMap := make(map[string]string)

		if m.Metadata.Spec.MetaData != nil {
			for key, value := range m.Metadata.Spec.MetaData {
				metadataMap[key], err = renderMetadata(value, tmpl)
				if err != nil {
					m.Log.Info("Failed to render Metadata", value)
					return err
				}
			}
		}

		m.Metadata.Status.Secrets[m3Machine.Name] = corev1.SecretReference{
			Name:      m3Machine.Name + "-metadata-" + machineIndex,
			Namespace: m.Metadata.Namespace,
		}

		marshalledMetadata, err := yaml.Marshal(metadataMap)
		if err != nil {
			m.Log.Info("Failed to marshal metadata")
			return err
		}

		err = createSecret(m.client, ctx, m3Machine.Name+"-metadata-"+machineIndex,
			m.Metadata.Namespace,
			m.Metadata.Labels[capi.ClusterLabelName], metaDataFinalizer,
			metav1.OwnerReference{
				Controller: pointer.BoolPtr(true),
				APIVersion: m.Metadata.APIVersion,
				Kind:       m.Metadata.Kind,
				Name:       m.Metadata.Name,
				UID:        m.Metadata.UID,
			},
			map[string][]byte{
				"metaData": marshalledMetadata,
			},
		)
		if err != nil {
			return err
		}
		m.Log.Info("Secret created", "Metal3machine", m3Machine.Name)
	}
	m.updateStatusTimestamp()
	return nil
}

func (m *MetadataManager) getM3Machine(cl client.Client, ctx context.Context,
	curOwnerRef metav1.OwnerReference,
) (*capm3.Metal3Machine, error) {
	tmpM3Machine := &capm3.Metal3Machine{}
	key := client.ObjectKey{
		Name:      curOwnerRef.Name,
		Namespace: m.Metadata.Namespace,
	}
	err := cl.Get(ctx, key, tmpM3Machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	if tmpM3Machine.Spec.MetaData.ConfigRef == nil {
		return nil, nil
	}
	if tmpM3Machine.Spec.MetaData.ConfigRef.Name != m.Metadata.Name {
		return nil, nil
	}
	if tmpM3Machine.Spec.MetaData.ConfigRef.Namespace == "" &&
		tmpM3Machine.Namespace != m.Metadata.Namespace {
		return nil, nil
	}
	if tmpM3Machine.Spec.MetaData.ConfigRef.Namespace != "" &&
		tmpM3Machine.Spec.MetaData.ConfigRef.Namespace != m.Metadata.Namespace {
		return nil, nil
	}
	return tmpM3Machine, nil
}

// DeleteSecrets deletes old secrets
func (m *MetadataManager) DeleteSecrets(ctx context.Context) error {
	for machineIndex, machineName := range m.Metadata.Status.Indexes {
		present := false
		for _, curOwnerRef := range m.Metadata.ObjectMeta.OwnerReferences {
			curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
			if err != nil {
				return err
			}
			if curOwnerRef.Kind != "Metal3Machine" ||
				curOwnerRefGV.Group != capm3.GroupVersion.Group {
				continue
			}
			if machineName == curOwnerRef.Name {
				present = true
				break
			}
		}
		if present {
			continue
		}
		m.Log.Info("Deleting metadata", "Metal3machine", machineName)
		dataSecret, ok := m.Metadata.Status.Secrets[machineName]
		if ok {
			if dataSecret.Name != "" {
				namespace := m.Metadata.Namespace
				if dataSecret.Namespace != "" {
					namespace = dataSecret.Namespace
				}
				m.Log.Info("Deleting Metadata secret for Metal3machine")
				err := deleteSecret(m.client, ctx, dataSecret.Name, namespace)
				if err != nil {
					return err
				}
			}
		}

		delete(m.Metadata.Status.Secrets, machineName)
		delete(m.Metadata.Status.Indexes, machineIndex)
		m.Log.Info("Metadata deleted", "Metal3machine", machineName)
	}
	m.updateStatusTimestamp()
	return nil
}

// DeleteRead returns true if the object is unreferenced (does not have
// Metal3Machine owner references)
func (m *MetadataManager) DeleteReady() (bool, error) {
	for _, curOwnerRef := range m.Metadata.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return false, err
		}
		if curOwnerRef.Kind == "Metal3Machine" ||
			curOwnerRefGV.Group == capm3.GroupVersion.Group {
			return false, nil
		}
	}
	m.Log.Info("Metal3Metadata ready for deletion")
	return true, nil
}

func IndexWithOffsetAndStep(index int, offset int, step int) (int, error) {
	return offset + index*step, nil
}

func getStr(value int, err error) (string, error) {
	return strconv.Itoa(value), err
}

func getHex(value int, err error) (string, error) {
	return fmt.Sprintf("%x", value), err
}

// createTemplate creates a template.Template object with the functions built in
func createTemplate(m3Machine *capm3.Metal3Machine, machineIndexStr string) (*template.Template, error) {
	machineName := ""
	machineIndex, err := strconv.Atoi(machineIndexStr)
	if err != nil {
		return nil, err
	}
	for _, ref := range m3Machine.ObjectMeta.OwnerReferences {
		if ref.Kind == "Machine" && ref.APIVersion == capi.GroupVersion.String() {
			machineName = ref.Name
			break
		}
	}
	getIndex := func() string { return machineIndexStr }
	getIndexWithOffset := func(offset int) (string, error) {
		return getStr(IndexWithOffsetAndStep(machineIndex, offset, 0))
	}
	getIndexWithStep := func(step int) (string, error) {
		return getStr(IndexWithOffsetAndStep(machineIndex, 0, step))
	}
	getIndexWithOffsetAndStep := func(offset int, step int) (string, error) {
		return getStr(IndexWithOffsetAndStep(machineIndex, offset, step))
	}
	getIndexWithStepAndOffset := func(step int, offset int) (string, error) {
		return getStr(IndexWithOffsetAndStep(machineIndex, offset, step))
	}
	getIndexHex := func() (string, error) {
		return getHex(machineIndex, nil)
	}
	getIndexWithOffsetHex := func(offset int) (string, error) {
		return getHex(IndexWithOffsetAndStep(machineIndex, offset, 0))
	}
	getIndexWithStepHex := func(step int) (string, error) {
		return getHex(IndexWithOffsetAndStep(machineIndex, 0, step))
	}
	getIndexWithOffsetAndStepHex := func(offset int, step int) (string, error) {
		return getHex(IndexWithOffsetAndStep(machineIndex, offset, step))
	}
	getIndexWithStepAndOffsetHex := func(step int, offset int) (string, error) {
		return getHex(IndexWithOffsetAndStep(machineIndex, offset, step))
	}
	getMachineName := func() string { return machineName }
	getMetal3MachineName := func() string { return m3Machine.Name }

	funcMap := template.FuncMap{
		"index":                     getIndex,
		"indexWithOffset":           getIndexWithOffset,
		"indexWithStep":             getIndexWithStep,
		"indexWithOffsetAndStep":    getIndexWithOffsetAndStep,
		"indexWithStepAndOffset":    getIndexWithStepAndOffset,
		"indexHex":                  getIndexHex,
		"indexWithOffsetHex":        getIndexWithOffsetHex,
		"indexWithStepHex":          getIndexWithStepHex,
		"indexWithOffsetAndStepHex": getIndexWithOffsetAndStepHex,
		"indexWithStepAndOffsetHex": getIndexWithStepAndOffsetHex,
		"machineName":               getMachineName,
		"metal3MachineName":         getMetal3MachineName,
	}
	return template.New("Metadata").Funcs(funcMap), nil
}

// renderMetadata renders a template and
func renderMetadata(value string, tmpl *template.Template) (string, error) {
	tmpl, err := tmpl.Parse(value)
	if err != nil {
		return "", err
	}
	tpl := bytes.Buffer{}
	if err := tmpl.Execute(&tpl, nil); err != nil {
		return "", err
	}
	return tpl.String(), nil
}
