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

	bmo "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DataTemplateManagerInterface is an interface for a DataTemplateManager
type DataTemplateManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	RecreateStatus(context.Context) error
	DeleteSecrets(context.Context) error
	CreateSecrets(context.Context) error
	DeleteReady() (bool, error)
}

// DataTemplateManager is responsible for performing machine reconciliation
type DataTemplateManager struct {
	client       client.Client
	DataTemplate *capm3.Metal3DataTemplate
	Log          logr.Logger
}

// NewDataTemplateManager returns a new helper for managing a dataTemplate object
func NewDataTemplateManager(client client.Client,
	dataTemplate *capm3.Metal3DataTemplate, dataTemplateLog logr.Logger) (*DataTemplateManager, error) {

	return &DataTemplateManager{
		client:       client,
		DataTemplate: dataTemplate,
		Log:          dataTemplateLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *DataTemplateManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.DataTemplate.Finalizers, capm3.DataTemplateFinalizer) {
		m.DataTemplate.Finalizers = append(m.DataTemplate.Finalizers,
			capm3.DataTemplateFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *DataTemplateManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	m.DataTemplate.Finalizers = Filter(m.DataTemplate.Finalizers,
		capm3.DataTemplateFinalizer,
	)
}

// RecreateStatus recreates the status if empty
func (m *DataTemplateManager) RecreateStatus(ctx context.Context) error {

	if m.DataTemplate.Status.LastUpdated != nil {
		return nil
	}

	if m.DataTemplate.Status.Indexes != nil &&
		(m.DataTemplate.Spec.MetaData == nil || m.DataTemplate.Status.MetaDataSecrets != nil) &&
		(m.DataTemplate.Spec.NetworkData == nil || m.DataTemplate.Status.NetworkDataSecrets != nil) {
		return nil
	}
	m.Log.Info("Recreating the Metal3DataTemplate status")
	m.DataTemplate.Status.Indexes = make(map[string]string)
	m.DataTemplate.Status.MetaDataSecrets = make(map[string]corev1.SecretReference)
	m.DataTemplate.Status.NetworkDataSecrets = make(map[string]corev1.SecretReference)

	for _, curOwnerRef := range m.DataTemplate.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return err
		}
		if curOwnerRef.Kind != "Metal3Machine" ||
			curOwnerRefGV.Group != capm3.GroupVersion.Group {
			continue
		}

		m.Log.Info("Verifying the owner", "Metal3machine", curOwnerRef.Name)
		// Verify that we have an owner ref machine that points to this DataTemplate
		m3Machine, err := m.getM3Machine(m.client, ctx, curOwnerRef)
		if err != nil {
			return err
		}
		if m3Machine == nil {
			continue
		}

		if m3Machine.Spec.MetaData == nil && m3Machine.Spec.NetworkData == nil {
			continue
		}
		re := regexp.MustCompile(`\d+$`)
		machineIndexMeta := ""
		machineIndexNetwork := ""
		if m.DataTemplate.Spec.MetaData != nil && m3Machine.Spec.MetaData != nil {
			machineIndexMeta = re.FindString(m3Machine.Spec.MetaData.Name)
		}
		if m.DataTemplate.Spec.NetworkData != nil && m3Machine.Spec.NetworkData != nil {
			machineIndexNetwork = re.FindString(m3Machine.Spec.NetworkData.Name)
		}
		if machineIndexMeta == "" && machineIndexNetwork == "" {
			continue
		}
		if machineIndexMeta != "" && machineIndexNetwork != "" &&
			machineIndexMeta != machineIndexNetwork {
			m.Log.Info("The secrets have different indexes on this machine",
				"Metal3machine", curOwnerRef.Name,
			)
			return errors.New("The secrets have different indexes on this machine")
		}
		machineIndex := machineIndexMeta
		if machineIndex == "" {
			machineIndex = machineIndexNetwork
		}
		m.DataTemplate.Status.Indexes[machineIndex] = curOwnerRef.Name
		m.Log.Info("Index added", "Metal3machine", curOwnerRef.Name)

		if err := m.populateSecretMap(ctx, "MetaData", curOwnerRef.Name, m3Machine.Spec.MetaData); err != nil {
			return err
		}
		if err := m.populateSecretMap(ctx, "NetworkData", curOwnerRef.Name, m3Machine.Spec.NetworkData); err != nil {
			return err
		}
		m.Log.Info("Secrets added", "Metal3machine", curOwnerRef.Name)
	}
	m.updateStatusTimestamp()
	return nil
}

func (m *DataTemplateManager) populateSecretMap(ctx context.Context,
	dataType, ownerName string, secretRef *corev1.SecretReference,
) error {
	if secretRef == nil {
		return nil
	}

	secretNamespace := m.DataTemplate.Namespace
	if secretRef.Namespace != "" {
		secretNamespace = secretRef.Namespace
	}
	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: secretNamespace,
	}
	err := m.client.Get(ctx, key, &tmpBootstrapSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			m.Log.Info("No secret found", "Metal3machine", ownerName)
			return nil
		}
		return err
	}
	switch dataType {
	case "MetaData":
		m.DataTemplate.Status.MetaDataSecrets[ownerName] = *secretRef
	case "NetworkData":
		m.DataTemplate.Status.NetworkDataSecrets[ownerName] = *secretRef
	default:
		return errors.New("Unknown data type")
	}
	return nil
}

func (m *DataTemplateManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.DataTemplate.Status.LastUpdated = &now
}

// CreateSecrets creates the missing secrets
func (m *DataTemplateManager) CreateSecrets(ctx context.Context) error {
	pendingItems := make(map[string]*capm3.Metal3Machine)
	bmhMap := make(map[string]*bmo.BareMetalHost)
	needsUpdate := false
	var err error

	for _, curOwnerRef := range m.DataTemplate.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return err
		}
		if curOwnerRef.Kind != "Metal3Machine" ||
			curOwnerRefGV.Group != capm3.GroupVersion.Group {
			continue
		}
		needsCreate := false
		secret, ok := m.DataTemplate.Status.MetaDataSecrets[curOwnerRef.Name]
		if m.DataTemplate.Spec.MetaData != nil && (!ok || secret.Name == "") {
			needsCreate = true
		}
		secret, ok = m.DataTemplate.Status.NetworkDataSecrets[curOwnerRef.Name]
		if m.DataTemplate.Spec.NetworkData != nil && (!ok || secret.Name == "") {
			needsCreate = true
		}
		if !needsCreate {
			continue
		}

		m.Log.Info("Verifying the owner", "Metal3machine", curOwnerRef.Name)

		// Verify that we have an owner ref machine that points to this DataTemplate
		m3Machine, err := m.getM3Machine(m.client, ctx, curOwnerRef)
		if err != nil {
			return err
		}
		if m3Machine == nil {
			continue
		}

		// Get the BaremetalHost associated
		bmh, err := getHost(ctx, m3Machine, m.client, m.Log)
		if err != nil {
			return err
		}
		if bmh == nil {
			continue
		}

		m.Log.Info("Getting index", "Metal3machine", curOwnerRef.Name)
		machineIndex := ""
		for key, value := range m.DataTemplate.Status.Indexes {
			if value == curOwnerRef.Name {
				machineIndex = key
				break
			}
		}

		if machineIndex == "" {
			machineIndexInt := len(m.DataTemplate.Status.Indexes)
			// The length of the map might be smaller than the highest index stored,
			// this means we have a gap to find
			for index := 0; index < len(m.DataTemplate.Status.Indexes); index++ {
				if _, ok := m.DataTemplate.Status.Indexes[strconv.Itoa(index)]; !ok {
					if machineIndexInt == len(m.DataTemplate.Status.Indexes) {
						machineIndexInt = index
						break
					}
				}
			}
			machineIndex = strconv.Itoa(machineIndexInt)

			if m.DataTemplate.Status.Indexes == nil {
				m.DataTemplate.Status.Indexes = make(map[string]string)
			}
			m.DataTemplate.Status.Indexes[machineIndex] = curOwnerRef.Name
			m.Log.Info("Index", "Metal3machine", curOwnerRef.Name, "index", machineIndex)
			needsUpdate = true
		}

		pendingItems[machineIndex] = m3Machine
		bmhMap[machineIndex] = bmh
	}
	if needsUpdate {
		err = updateObject(m.client, ctx, m.DataTemplate)
		if err != nil {
			return err
		}
	}

	if m.DataTemplate.Status.MetaDataSecrets == nil {
		m.DataTemplate.Status.MetaDataSecrets = make(map[string]corev1.SecretReference)
	}
	if m.DataTemplate.Status.NetworkDataSecrets == nil {
		m.DataTemplate.Status.NetworkDataSecrets = make(map[string]corev1.SecretReference)
	}

	for machineIndex, m3Machine := range pendingItems {
		bmh := bmhMap[machineIndex]
		m.Log.Info("Creating secret", "Metal3machine", m3Machine.Name)
		tmpl, err := createTemplate(m3Machine, machineIndex, bmh)
		if err != nil {
			return err
		}

		if err := m.generateSecret(ctx, m3Machine.Name,
			m3Machine.Name+"-metadata-"+machineIndex,
			"metaData", m.DataTemplate.Spec.MetaData, tmpl,
		); err != nil {
			return err
		}
		if err := m.generateSecret(ctx, m3Machine.Name,
			m3Machine.Name+"-networkdata-"+machineIndex,
			"networkData", m.DataTemplate.Spec.NetworkData, tmpl,
		); err != nil {
			return err
		}
		m.Log.Info("Secrets created", "Metal3machine", m3Machine.Name)
	}
	m.updateStatusTimestamp()
	return nil
}

func (m *DataTemplateManager) generateSecret(ctx context.Context, machineName,
	secretName, secretKey string, valueTemplate *string, tmpl *template.Template,
) error {
	if valueTemplate == nil {
		return nil
	}
	renderedValue, err := renderDataTemplate(*valueTemplate, tmpl)
	if err != nil {
		m.Log.Info("Failed to render template")
		return err
	}
	err = createSecret(m.client, ctx, secretName,
		m.DataTemplate.Namespace,
		m.DataTemplate.Labels[capi.ClusterLabelName],
		metav1.OwnerReference{
			Controller: pointer.BoolPtr(true),
			APIVersion: m.DataTemplate.APIVersion,
			Kind:       m.DataTemplate.Kind,
			Name:       m.DataTemplate.Name,
			UID:        m.DataTemplate.UID,
		},
		map[string][]byte{
			secretKey: []byte(renderedValue),
		},
	)
	if err != nil {
		return err
	}
	secretRef := corev1.SecretReference{
		Name:      secretName,
		Namespace: m.DataTemplate.Namespace,
	}
	switch secretKey {
	case "metaData":
		m.DataTemplate.Status.MetaDataSecrets[machineName] = secretRef
	case "networkData":
		m.DataTemplate.Status.NetworkDataSecrets[machineName] = secretRef
	default:
		return errors.New("Unknown data type")
	}
	return nil
}

func (m *DataTemplateManager) getM3Machine(cl client.Client, ctx context.Context,
	curOwnerRef metav1.OwnerReference,
) (*capm3.Metal3Machine, error) {
	tmpM3Machine := &capm3.Metal3Machine{}
	key := client.ObjectKey{
		Name:      curOwnerRef.Name,
		Namespace: m.DataTemplate.Namespace,
	}
	err := cl.Get(ctx, key, tmpM3Machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	if tmpM3Machine.Spec.DataTemplate == nil {
		return nil, nil
	}
	if tmpM3Machine.Spec.DataTemplate.Name != m.DataTemplate.Name {
		return nil, nil
	}
	if tmpM3Machine.Spec.DataTemplate.Namespace == "" &&
		tmpM3Machine.Namespace != m.DataTemplate.Namespace {
		return nil, nil
	}
	if tmpM3Machine.Spec.DataTemplate.Namespace != "" &&
		tmpM3Machine.Spec.DataTemplate.Namespace != m.DataTemplate.Namespace {
		return nil, nil
	}
	return tmpM3Machine, nil
}

// DeleteSecrets deletes old secrets
func (m *DataTemplateManager) DeleteSecrets(ctx context.Context) error {
	for machineIndex, machineName := range m.DataTemplate.Status.Indexes {
		present := false
		for _, curOwnerRef := range m.DataTemplate.ObjectMeta.OwnerReferences {
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

		if err := m.deleteSecret(ctx, "metaData", machineName); err != nil {
			return err
		}
		if err := m.deleteSecret(ctx, "networkData", machineName); err != nil {
			return err
		}

		delete(m.DataTemplate.Status.Indexes, machineIndex)
		m.Log.Info("DataTemplate deleted", "Metal3machine", machineName)
	}
	m.updateStatusTimestamp()
	return nil
}

func (m *DataTemplateManager) deleteSecret(ctx context.Context, dataType,
	machineName string,
) error {
	var secretMap map[string]corev1.SecretReference
	switch dataType {
	case "metaData":
		secretMap = m.DataTemplate.Status.MetaDataSecrets
	case "networkData":
		secretMap = m.DataTemplate.Status.NetworkDataSecrets
	default:
		return errors.New("Unknwon data type")
	}
	if secretMap == nil {
		return nil
	}
	dataSecret, ok := secretMap[machineName]
	if ok {
		if dataSecret.Name != "" {
			namespace := m.DataTemplate.Namespace
			if dataSecret.Namespace != "" {
				namespace = dataSecret.Namespace
			}
			err := deleteSecret(m.client, ctx, dataSecret.Name, namespace)
			if err != nil {
				return err
			}
		}
	}
	delete(secretMap, machineName)
	return nil
}

// DeleteRead returns true if the object is unreferenced (does not have
// Metal3Machine owner references)
func (m *DataTemplateManager) DeleteReady() (bool, error) {
	for _, curOwnerRef := range m.DataTemplate.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return false, err
		}
		if curOwnerRef.Kind == "Metal3Machine" ||
			curOwnerRefGV.Group == capm3.GroupVersion.Group {
			return false, nil
		}
	}
	m.Log.Info("Metal3DataTemplate ready for deletion")
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
func createTemplate(m3Machine *capm3.Metal3Machine, machineIndexStr string,
	bmh *bmo.BareMetalHost,
) (*template.Template, error) {
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
		return getStr(IndexWithOffsetAndStep(machineIndex, offset, 1))
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
		return getHex(IndexWithOffsetAndStep(machineIndex, offset, 1))
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
	getBMHName := func() string { return bmh.Name }
	getBMHMacByName := func(name string) (string, error) {
		if bmh.Status.HardwareDetails == nil || bmh.Status.HardwareDetails.NIC == nil {
			return "", errors.New("Nics list not populated")
		}
		for _, nics := range bmh.Status.HardwareDetails.NIC {
			if nics.Name == name {
				return nics.MAC, nil
			}
		}
		return "", errors.New(fmt.Sprintf("Nic name not found %v", name))
	}

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
		"bareMetalHostName":         getBMHName,
		"bareMetalHostMACByName":    getBMHMacByName,
	}
	return template.New("DataTemplate").Funcs(funcMap), nil
}

// renderDataTemplate renders a template and
func renderDataTemplate(value string, tmpl *template.Template) (string, error) {
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
