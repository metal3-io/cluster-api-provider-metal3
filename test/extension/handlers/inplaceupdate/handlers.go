// Package inplaceupdate contains the handlers for the in-place update hooks.
//
// The implementation of the handlers is specifically designed for Cluster API Metal3 E2E test use cases.
// When implementing custom RuntimeExtension, it is only required to expose HandlerFunc with the
// signature defined in sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1.
package inplaceupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	infrav1beta1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get

const (
	trueString                      = "true"
	extensionUpdatingMachineMessage = "Extension is updating Machine"
)

// ExtensionHandlers provides a common struct shared across the in-place update hook handlers.
type ExtensionHandlers struct {
	decoder runtime.Decoder
	client  client.Client
	state   sync.Map
}

// NewExtensionHandlers returns a new ExtensionHandlers for the in-place update hook handlers.
func NewExtensionHandlers(client client.Client) *ExtensionHandlers {
	scheme := runtime.NewScheme()
	_ = infrav1beta1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &ExtensionHandlers{
		client: client,
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			infrav1beta1.GroupVersion,
			bootstrapv1.GroupVersion,
			controlplanev1.GroupVersion,
		),
	}
}

// * MachineSpec.Version.
func canUpdateMachineSpec(current, desired *clusterv1.MachineSpec) {
	if current.Version != desired.Version {
		current.Version = desired.Version
	}
}

// canUpdateKubeadmConfigSpec declares that this extension can update.
func canUpdateKubeadmConfigSpec(_, _ *bootstrapv1.KubeadmConfigSpec) {
	// Todo: add more fields as needed.
}

// * Metal3MachineSpec.DataTemplate.
func canUpdateMetal3MachineSpec(current, desired *infrav1beta1.Metal3MachineSpec) {
	if current.Image != desired.Image {
		current.Image = desired.Image
	}
	if current.DataTemplate != desired.DataTemplate {
		current.DataTemplate = desired.DataTemplate
	}
}

// DoCanUpdateMachine implements the CanUpdateMachine hook.
// This function create the patches to allow in-place updates on certain fields
// in Machine. If in-place update of the field is not supported, the field is
// left unchanged, and empty patches will be returned, causing Cluster API to
// perform roll-out updates.
// This is for KCP in-place updates.
func (h *ExtensionHandlers) DoCanUpdateMachine(ctx context.Context, req *runtimehooksv1.CanUpdateMachineRequest, resp *runtimehooksv1.CanUpdateMachineResponse) {
	log := ctrl.LoggerFrom(ctx).WithValues("Machine", klog.KObj(&req.Desired.Machine))
	log.Info("CanUpdateMachine is called")

	if req.Settings["disableInPlaceUpdates"] == trueString {
		resp.Status = runtimehooksv1.ResponseStatusSuccess
		return
	}

	currentMachine, desiredMachine,
		currentBootstrapConfig, desiredBootstrapConfig,
		currentInfraMachine, desiredInfraMachine, err := h.getObjectsFromCanUpdateMachineRequest(req)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = err.Error()
		return
	}

	log.Info("Current Machine", "machine", currentMachine)
	log.Info("Desired Machine", "machine", desiredMachine)

	log.Info("Current currentBootstrapConfig", "currentBootstrapConfig", currentBootstrapConfig)
	log.Info("Desired desiredBootstrapConfig", "desiredBootstrapConfig", desiredBootstrapConfig)

	log.Info("Current currentInfraMachine", "currentInfraMachine", currentInfraMachine)
	log.Info("Desired desiredInfraMachine", "desiredInfraMachine", desiredInfraMachine)

	// Declare changes that this Runtime Extension can update in-place.

	// Machine
	canUpdateMachineSpec(&currentMachine.Spec, &desiredMachine.Spec)

	// BootstrapConfig (we can only update KubeadmConfigs)
	currentKubeadmConfig, isCurrentKubeadmConfig := currentBootstrapConfig.(*bootstrapv1.KubeadmConfig)
	desiredKubeadmConfig, isDesiredKubeadmConfig := desiredBootstrapConfig.(*bootstrapv1.KubeadmConfig)
	if isCurrentKubeadmConfig && isDesiredKubeadmConfig {
		canUpdateKubeadmConfigSpec(&currentKubeadmConfig.Spec, &desiredKubeadmConfig.Spec)
	}

	// InfraMachine (we can only update Metal3Machines)
	currentMetal3Machine, isCurrentMetal3Machine := currentInfraMachine.(*infrav1beta1.Metal3Machine)
	desiredMetal3Machine, isDesiredMetal3Machine := desiredInfraMachine.(*infrav1beta1.Metal3Machine)
	if isCurrentMetal3Machine && isDesiredMetal3Machine {
		canUpdateMetal3MachineSpec(&currentMetal3Machine.Spec, &desiredMetal3Machine.Spec)
	}

	if err := h.computeCanUpdateMachineResponse(req, resp, currentMachine, currentBootstrapConfig, currentInfraMachine); err != nil {
		log.Info("Failed to compute CanUpdateMachine response", "error", err)
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = err.Error()
		return
	}

	log.Info("Successfully computed CanUpdateMachine response")

	resp.Status = runtimehooksv1.ResponseStatusSuccess
}

// DoCanUpdateMachineSet implements the CanUpdateMachineSet hook.
// This function create the patches to allow in-place updates on certain fields
// in MachineSet. If in-place update of the field is not supported, the field is
// left unchanged, and empty patches will be returned, causing Cluster API to
// perform roll-out updates.
// This is for worker machines in-place updates.
func (h *ExtensionHandlers) DoCanUpdateMachineSet(ctx context.Context, req *runtimehooksv1.CanUpdateMachineSetRequest, resp *runtimehooksv1.CanUpdateMachineSetResponse) {
	log := ctrl.LoggerFrom(ctx).WithValues("MachineSet", klog.KObj(&req.Desired.MachineSet))
	log.Info("CanUpdateMachineSet is called")

	if req.Settings["disableInPlaceUpdates"] == trueString {
		resp.Status = runtimehooksv1.ResponseStatusSuccess
		return
	}

	currentMachineSet, desiredMachineSet,
		currentBootstrapConfigTemplate, desiredBootstrapConfigTemplate,
		currentInfraMachineTemplate, desiredInfraMachineTemplate, err := h.getObjectsFromCanUpdateMachineSetRequest(req)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = err.Error()
		return
	}

	log.Info("Current MachineSet", "machineSet", currentMachineSet)
	log.Info("Desired MachineSet", "machineSet", desiredMachineSet)

	// Declare changes that this Runtime Extension can update in-place.

	// Machine
	canUpdateMachineSpec(&currentMachineSet.Spec.Template.Spec, &desiredMachineSet.Spec.Template.Spec)

	// BootstrapConfig (we can only update KubeadmConfigs)
	currentKubeadmConfigTemplate, isCurrentKubeadmConfigTemplate := currentBootstrapConfigTemplate.(*bootstrapv1.KubeadmConfigTemplate)
	desiredKubeadmConfigTemplate, isDesiredKubeadmConfigTemplate := desiredBootstrapConfigTemplate.(*bootstrapv1.KubeadmConfigTemplate)
	if isCurrentKubeadmConfigTemplate && isDesiredKubeadmConfigTemplate {
		canUpdateKubeadmConfigSpec(&currentKubeadmConfigTemplate.Spec.Template.Spec, &desiredKubeadmConfigTemplate.Spec.Template.Spec)
	}

	// InfraMachine (we can only update Metal3Machines)
	currentMetal3MachineTemplate, isCurrentMetal3MachineTemplate := currentInfraMachineTemplate.(*infrav1beta1.Metal3MachineTemplate)
	desiredMetal3MachineTemplate, isDesiredMetal3MachineTemplate := desiredInfraMachineTemplate.(*infrav1beta1.Metal3MachineTemplate)
	if isCurrentMetal3MachineTemplate && isDesiredMetal3MachineTemplate {
		canUpdateMetal3MachineSpec(&currentMetal3MachineTemplate.Spec.Template.Spec, &desiredMetal3MachineTemplate.Spec.Template.Spec)
	}

	if err := h.computeCanUpdateMachineSetResponse(req, resp, currentMachineSet, currentBootstrapConfigTemplate, currentInfraMachineTemplate); err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = err.Error()
		return
	}

	resp.Status = runtimehooksv1.ResponseStatusSuccess
}

// DoUpdateMachine implements the UpdateMachine hook.
// It performs an actual in-place Kubernetes upgrade by SSHing to the machine.
func (h *ExtensionHandlers) DoUpdateMachine(ctx context.Context, req *runtimehooksv1.UpdateMachineRequest, resp *runtimehooksv1.UpdateMachineResponse) {
	log := ctrl.LoggerFrom(ctx).WithValues("Machine", klog.KObj(&req.Desired.Machine))
	log.Info("UpdateMachine is called")
	defer func() {
		log.Info("UpdateMachine response", "Machine", klog.KObj(&req.Desired.Machine), "status", resp.Status, "message", resp.Message, "retryAfterSeconds", resp.RetryAfterSeconds)
	}()

	if req.Settings["disableInPlaceUpdates"] == trueString {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "Unexpected call to UpdateMachine hook after CanUpdateMachine or CanUpdateMachineSet did not return any patches"
		return
	}

	key := klog.KObj(&req.Desired.Machine).String()
	errorKey := key + "errored"

	// Get the machine IP address (pass empty string to match any IP pool)
	machineIP, err := h.getMachineIP(ctx, &req.Desired.Machine, req.Desired.Machine.Spec.ClusterName+"-baremetalv4-pool")
	if err != nil {
		log.Error(err, "Failed to get machine IP address")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("Failed to get machine IP: %v", err)
		return
	}

	// Get target version from the desired machine spec
	targetVersion := ""
	if req.Desired.Machine.Spec.Version != "" {
		targetVersion = strings.TrimPrefix(req.Desired.Machine.Spec.Version, "v")
	}

	if targetVersion == "" {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "Target Kubernetes version not specified"
		return
	}

	// If a previous async run failed, surface error and cleanup state
	if _, failed := h.state.Load(errorKey); failed {
		h.state.Delete(errorKey)
		h.state.Delete(key)
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "Failed to upgrade Kubernetes: background upgrade errored; check logs on target node"
		return
	}

	user := "metal3"
	desiredBootstrapConfig, _, err := h.decoder.Decode(req.Desired.BootstrapConfig.Raw, nil, req.Desired.BootstrapConfig.Object)
	if err != nil {
		log.Error(err, "Failed to get desired bootstrap config")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("Failed to get desired bootstrap config: %v", err)
		return
	}
	desiredKubeadmConfig, isDesiredKubeadmConfig := desiredBootstrapConfig.(*bootstrapv1.KubeadmConfig)
	if isDesiredKubeadmConfig {
		user = desiredKubeadmConfig.Spec.Users[0].Name
	}

	// If upgrade already started, verify completion by checking the node version
	if _, started := h.state.Load(key); started {
		log.Info("Checking if Kubernetes is upgraded", "machineIP", machineIP, "targetVersion", targetVersion)
		isUpgraded, err := isKubernetesUpgraded(machineIP, user, targetVersion)
		if err != nil {
			log.Error(err, "Failed to check if Kubernetes is upgraded")
			resp.Status = runtimehooksv1.ResponseStatusSuccess
			resp.Message = extensionUpdatingMachineMessage
			resp.RetryAfterSeconds = 15
			return
		}
		if isUpgraded {
			h.state.Delete(key)
			resp.Status = runtimehooksv1.ResponseStatusSuccess
			resp.Message = "Extension completed updating Machine"
			resp.RetryAfterSeconds = 0
			return
		}
		// Still in progress
		resp.Status = runtimehooksv1.ResponseStatusSuccess
		resp.Message = extensionUpdatingMachineMessage
		resp.RetryAfterSeconds = 15
		return
	}

	// First time called - start the upgrade process in background
	log.Info("Starting in-place Kubernetes upgrade", "machineIP", machineIP, "targetVersion", targetVersion)
	h.state.Store(key, true)
	go func(machineIP, user, targetVersion, errKey string) {
		if err := upgradeKubernetesInPlace(machineIP, user, targetVersion); err != nil {
			log.Error(err, "Background upgrade failed")
			h.state.Store(errKey, true)
		}
	}(machineIP, user, targetVersion, errorKey)

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	resp.Message = extensionUpdatingMachineMessage
	resp.RetryAfterSeconds = 15
}

//nolint:dupl // Similar logic to getObjectsFromCanUpdateMachineSetRequest but operates on different types
func (h *ExtensionHandlers) getObjectsFromCanUpdateMachineRequest(req *runtimehooksv1.CanUpdateMachineRequest) (*clusterv1.Machine, *clusterv1.Machine, runtime.Object, runtime.Object, runtime.Object, runtime.Object, error) { //nolint:gocritic // accepting high number of return parameters for now
	currentMachine := req.Current.Machine.DeepCopy()
	desiredMachine := req.Desired.Machine.DeepCopy()
	currentBootstrapConfig, _, err := h.decoder.Decode(req.Current.BootstrapConfig.Raw, nil, req.Current.BootstrapConfig.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	desiredBootstrapConfig, _, err := h.decoder.Decode(req.Desired.BootstrapConfig.Raw, nil, req.Desired.BootstrapConfig.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	currentInfraMachine, _, err := h.decoder.Decode(req.Current.InfrastructureMachine.Raw, nil, req.Current.InfrastructureMachine.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	desiredInfraMachine, _, err := h.decoder.Decode(req.Desired.InfrastructureMachine.Raw, nil, req.Desired.InfrastructureMachine.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return currentMachine, desiredMachine, currentBootstrapConfig, desiredBootstrapConfig, currentInfraMachine, desiredInfraMachine, nil
}

func (h *ExtensionHandlers) computeCanUpdateMachineResponse(req *runtimehooksv1.CanUpdateMachineRequest, resp *runtimehooksv1.CanUpdateMachineResponse, currentMachine *clusterv1.Machine, currentBootstrapConfig, currentInfraMachine runtime.Object) error {
	marshalledCurrentMachine, err := json.Marshal(req.Current.Machine)
	if err != nil {
		return err
	}
	machinePatch, err := createJSONPatch(marshalledCurrentMachine, currentMachine)
	if err != nil {
		return err
	}
	bootstrapConfigPatch, err := createJSONPatch(req.Current.BootstrapConfig.Raw, currentBootstrapConfig)
	if err != nil {
		return err
	}
	infraMachinePatch, err := createJSONPatch(req.Current.InfrastructureMachine.Raw, currentInfraMachine)
	if err != nil {
		return err
	}

	resp.MachinePatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     machinePatch,
	}
	resp.BootstrapConfigPatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     bootstrapConfigPatch,
	}
	resp.InfrastructureMachinePatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     infraMachinePatch,
	}
	return nil
}

// getMachineIP retrieves the IP address of a machine using the chain: Machine -> Metal3Machine -> Metal3Data -> IPAddress.
// This follows the same approach as MachineToIPAddress1beta1 from the e2e tests.
func (h *ExtensionHandlers) getMachineIP(ctx context.Context, machine *clusterv1.Machine, ippoolName string) (string, error) {
	// Get the Metal3Machine
	metal3Machine := &infrav1beta1.Metal3Machine{}
	err := h.client.Get(ctx, types.NamespacedName{
		Namespace: machine.Namespace,
		Name:      machine.Spec.InfrastructureRef.Name,
	}, metal3Machine)
	if err != nil {
		return "", fmt.Errorf("couldn't get Metal3Machine %s/%s: %w", machine.Namespace, machine.Spec.InfrastructureRef.Name, err)
	}

	// List Metal3Data objects and find the one owned by this Metal3Machine
	m3DataList := &infrav1beta1.Metal3DataList{}
	err = h.client.List(ctx, m3DataList, client.InNamespace(machine.Namespace))
	if err != nil {
		return "", fmt.Errorf("couldn't list Metal3Data objects: %w", err)
	}

	var m3Data *infrav1beta1.Metal3Data
	for i, m3d := range m3DataList.Items {
		for _, owner := range m3d.OwnerReferences {
			if owner.Name == metal3Machine.Name {
				m3Data = &m3DataList.Items[i]
				break
			}
		}
		if m3Data != nil {
			break
		}
	}
	if m3Data == nil {
		return "", errors.New("couldn't find matching Metal3Data object")
	}

	// List IPAddress objects and find the one owned by this Metal3Data
	IPAddresses := &ipamv1.IPAddressList{}
	err = h.client.List(ctx, IPAddresses, client.InNamespace(machine.Namespace))
	if err != nil {
		return "", fmt.Errorf("couldn't list IPAddress objects: %w", err)
	}

	var IPAddress *ipamv1.IPAddress
	for i, ip := range IPAddresses.Items {
		for _, owner := range ip.OwnerReferences {
			if owner.Name == m3Data.Name && (ippoolName == "" || ip.Spec.Pool.Name == ippoolName) {
				IPAddress = &IPAddresses.Items[i]
				break
			}
		}
		if IPAddress != nil {
			break
		}
	}
	if IPAddress == nil {
		return "", errors.New("couldn't find matching IPAddress object")
	}

	return string(IPAddress.Spec.Address), nil
}

//nolint:dupl // Similar logic to getObjectsFromCanUpdateMachineRequest but operates on different types
func (h *ExtensionHandlers) getObjectsFromCanUpdateMachineSetRequest(req *runtimehooksv1.CanUpdateMachineSetRequest) (*clusterv1.MachineSet, *clusterv1.MachineSet, runtime.Object, runtime.Object, runtime.Object, runtime.Object, error) { //nolint:gocritic // accepting high number of return parameters for now
	currentMachineSet := req.Current.MachineSet.DeepCopy()
	desiredMachineSet := req.Desired.MachineSet.DeepCopy()
	currentBootstrapConfigTemplate, _, err := h.decoder.Decode(req.Current.BootstrapConfigTemplate.Raw, nil, req.Current.BootstrapConfigTemplate.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	desiredBootstrapConfigTemplate, _, err := h.decoder.Decode(req.Desired.BootstrapConfigTemplate.Raw, nil, req.Desired.BootstrapConfigTemplate.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	currentInfraMachineTemplate, _, err := h.decoder.Decode(req.Current.InfrastructureMachineTemplate.Raw, nil, req.Current.InfrastructureMachineTemplate.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	desiredInfraMachineTemplate, _, err := h.decoder.Decode(req.Desired.InfrastructureMachineTemplate.Raw, nil, req.Desired.InfrastructureMachineTemplate.Object)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return currentMachineSet, desiredMachineSet, currentBootstrapConfigTemplate, desiredBootstrapConfigTemplate, currentInfraMachineTemplate, desiredInfraMachineTemplate, nil
}

func (h *ExtensionHandlers) computeCanUpdateMachineSetResponse(req *runtimehooksv1.CanUpdateMachineSetRequest, resp *runtimehooksv1.CanUpdateMachineSetResponse, currentMachineSet *clusterv1.MachineSet, currentBootstrapConfigTemplate, currentInfraMachineTemplate runtime.Object) error {
	marshalledCurrentMachineSet, err := json.Marshal(req.Current.MachineSet)
	if err != nil {
		return err
	}
	machineSetPatch, err := createJSONPatch(marshalledCurrentMachineSet, currentMachineSet)
	if err != nil {
		return err
	}
	bootstrapConfigTemplatePatch, err := createJSONPatch(req.Current.BootstrapConfigTemplate.Raw, currentBootstrapConfigTemplate)
	if err != nil {
		return err
	}
	infraMachineTemplatePatch, err := createJSONPatch(req.Current.InfrastructureMachineTemplate.Raw, currentInfraMachineTemplate)
	if err != nil {
		return err
	}

	resp.MachineSetPatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     machineSetPatch,
	}
	resp.BootstrapConfigTemplatePatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     bootstrapConfigTemplatePatch,
	}
	resp.InfrastructureMachineTemplatePatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     infraMachineTemplatePatch,
	}
	return nil
}

// createJSONPatch creates a RFC 6902 JSON patch from the original and the modified object.
func createJSONPatch(marshalledOriginal []byte, modified runtime.Object) ([]byte, error) {
	// TODO: avoid producing patches for status (although they will be ignored by the KCP / MD controllers anyway)
	marshalledModified, err := json.Marshal(modified)
	if err != nil {
		return nil, errors.Errorf("failed to marshal modified object: %v", err)
	}

	patch, err := jsonpatch.CreatePatch(marshalledOriginal, marshalledModified)
	if err != nil {
		return nil, errors.Errorf("failed to create patch: %v", err)
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, errors.Errorf("failed to marshal patch: %v", err)
	}

	return patchBytes, nil
}
