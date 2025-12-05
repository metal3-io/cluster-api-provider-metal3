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
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	m3machine         = "metal3machine"
	host              = "baremetalhost"
	capimachine       = "machine"
	DataLabelName     = "infrastructure.cluster.x-k8s.io/data-name"
	PoolLabelName     = "infrastructure.cluster.x-k8s.io/pool-name"
	networkDataSuffix = "-networkdata"
	metaDataSuffix    = "-metadata"
	IPPoolKind        = "IPPool"
)

var (
	EnableBMHNameBasedPreallocation bool
)

// DataManagerInterface is an interface for a DataManager.
type DataManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	Reconcile(ctx context.Context) error
	ReleaseLeases(ctx context.Context) error
}

// DataManager is responsible for performing machine reconciliation.
type DataManager struct {
	client client.Client
	Data   *infrav1.Metal3Data
	Log    logr.Logger
}

// NewDataManager returns a new helper for managing a Metal3Data object.
func NewDataManager(client client.Client,
	data *infrav1.Metal3Data, dataLog logr.Logger) (*DataManager, error) {
	return &DataManager{
		client: client,
		Data:   data,
		Log:    dataLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *DataManager) SetFinalizer() {
	// If the Metal3Data doesn't have finalizer, add it.
	if !controllerutil.ContainsFinalizer(m.Data, infrav1.DataFinalizer) {
		controllerutil.AddFinalizer(m.Data, infrav1.DataFinalizer)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *DataManager) UnsetFinalizer() {
	// Remove the finalizer.
	controllerutil.RemoveFinalizer(m.Data, infrav1.DataFinalizer)
}

// clearError clears error message from Metal3Data status.
func (m *DataManager) clearError(_ context.Context) {
	m.Data.Status.ErrorMessage = nil
}

// setError sets error message to Metal3Data status.
func (m *DataManager) setError(_ context.Context, msg string) {
	m.Data.Status.ErrorMessage = &msg
}

// Reconcile handles Metal3Data events.
func (m *DataManager) Reconcile(ctx context.Context) error {
	m.clearError(ctx)

	if err := m.createSecrets(ctx); err != nil {
		var reconcileError ReconcileError
		if errors.As(err, &reconcileError) && reconcileError.IsTransient() {
			return err
		}
		m.setError(ctx, errors.Cause(err).Error())
		return err
	}

	return nil
}

// CreateSecrets creates the secret if they do not exist.
func (m *DataManager) createSecrets(ctx context.Context) error {
	var metaDataErr, networkDataErr error

	if m.Data.Spec.Template.Name == "" {
		return nil
	}
	if m.Data.Spec.Template.Namespace == "" {
		m.Data.Spec.Template.Namespace = m.Data.Namespace
	}
	// Fetch the Metal3DataTemplate object to get the templates
	m3dt, err := fetchM3DataTemplate(ctx, &m.Data.Spec.Template, m.client,
		m.Log, m.Data.Labels[clusterv1.ClusterNameLabel],
	)
	if err != nil {
		return err
	}
	if m3dt == nil {
		return nil
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched Metal3DataTemplate")

	// Fetch the Metal3Machine, to get the related info
	m3m, err := m.getM3Machine(ctx, m3dt)
	if err != nil {
		return err
	}
	if m3m == nil {
		return errors.New("Metal3Machine associated with Metal3DataTemplate is not found")
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched Metal3Machine")

	// If the MetaData is given as part of Metal3DataTemplate
	if m3dt.Spec.MetaData != nil {
		m.Log.Info("Metadata is part of Metal3DataTemplate")
		// If the secret name is unset, set it
		if m.Data.Spec.MetaData == nil || m.Data.Spec.MetaData.Name == "" {
			m.Data.Spec.MetaData = &corev1.SecretReference{
				Name:      m3m.Name + metaDataSuffix,
				Namespace: m.Data.Namespace,
			}
		}

		// Try to fetch the secret. If it exists, we do not modify it, to be able
		// to reprovision a node in the exact same state.
		m.Log.Info("Checking if secret exists", "secret", m.Data.Spec.MetaData.Name)
		_, metaDataErr = checkSecretExists(ctx, m.client, m.Data.Spec.MetaData.Name,
			m.Data.Namespace,
		)

		if metaDataErr != nil && !apierrors.IsNotFound(metaDataErr) {
			return metaDataErr
		}
		if apierrors.IsNotFound(metaDataErr) {
			m.Log.Info("MetaData secret creation needed", "secret", m.Data.Spec.MetaData.Name)
		}
	}

	// If the NetworkData is given as part of Metal3DataTemplate
	if m3dt.Spec.NetworkData != nil {
		m.Log.Info("NetworkData is part of Metal3DataTemplate")
		// If the secret name is unset, set it
		if m.Data.Spec.NetworkData == nil || m.Data.Spec.NetworkData.Name == "" {
			m.Data.Spec.NetworkData = &corev1.SecretReference{
				Name:      m3m.Name + networkDataSuffix,
				Namespace: m.Data.Namespace,
			}
		}

		// Try to fetch the secret. If it exists, we do not modify it, to be able
		// to reprovision a node in the exact same state.
		m.Log.Info("Checking if secret exists", "secret", m.Data.Spec.NetworkData.Name)
		_, networkDataErr = checkSecretExists(ctx, m.client, m.Data.Spec.NetworkData.Name,
			m.Data.Namespace,
		)
		if networkDataErr != nil && !apierrors.IsNotFound(networkDataErr) {
			return networkDataErr
		}
		if apierrors.IsNotFound(networkDataErr) {
			m.Log.Info("NetworkData secret creation needed", "secret", m.Data.Spec.NetworkData.Name)
		}
	}

	// No secret needs creation
	if metaDataErr == nil && networkDataErr == nil {
		m.Log.Info("Metal3Data Reconciled")
		m.Data.Status.Ready = true
		return nil
	}

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, m.client, m3m.ObjectMeta)

	if err != nil {
		return errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		errMessage := "Waiting for Machine Controller to set OwnerRef on Metal3Machine"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched Machine")

	// Fetch the BMH associated with the M3M
	bmh, err := getHost(ctx, m3m, m.client, m.Log)
	if err != nil {
		return err
	}
	if bmh == nil {
		errMessage := "Waiting for BareMetalHost to become available"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched BMH")

	// Fetch all the Metal3IPPools and create Metal3IPClaims as needed. Check if the
	// IP address has been allocated, if so, fetch the address, gateway and prefix.
	poolAddresses, err := m.getAddressesFromPool(ctx, *m3dt, m3m, capiMachine, bmh)
	if err != nil {
		return err
	}

	// Create the owner Ref for the secret
	ownerRefs := []metav1.OwnerReference{
		{
			Controller: ptr.To(true),
			APIVersion: m.Data.APIVersion,
			Kind:       m.Data.Kind,
			Name:       m.Data.Name,
			UID:        m.Data.UID,
		},
	}

	// The MetaData secret must be created
	if apierrors.IsNotFound(metaDataErr) {
		m.Log.Info("Creating Metadata secret")
		metadata, err := renderMetaData(m.Data, m3dt, m3m, capiMachine, bmh, poolAddresses)
		if err != nil {
			return err
		}
		if err := createSecret(ctx, m.client, m.Data.Spec.MetaData.Name,
			m.Data.Namespace, m3dt.Labels[clusterv1.ClusterNameLabel],
			ownerRefs, map[string][]byte{"metaData": metadata},
		); err != nil {
			return err
		}
	}

	// The NetworkData secret must be created
	if apierrors.IsNotFound(networkDataErr) {
		m.Log.Info("Creating Networkdata secret")
		networkData, err := renderNetworkData(m3dt, m3m, capiMachine, bmh, poolAddresses)
		if err != nil {
			return err
		}
		if err := createSecret(ctx, m.client, m.Data.Spec.NetworkData.Name,
			m.Data.Namespace, m3dt.Labels[clusterv1.ClusterNameLabel],
			ownerRefs, map[string][]byte{"networkData": networkData},
		); err != nil {
			return err
		}
	}

	m.Log.Info("Metal3Data reconciled")
	m.Data.Status.Ready = true
	return nil
}

// ReleaseLeases releases addresses from pool.
func (m *DataManager) ReleaseLeases(ctx context.Context) error {
	if m.Data.Spec.Template.Name == "" {
		return nil
	}
	if m.Data.Spec.Template.Namespace == "" {
		m.Data.Spec.Template.Namespace = m.Data.Namespace
	}
	// Fetch the Metal3DataTemplate object to get the templates
	m3dt, err := fetchM3DataTemplate(ctx, &m.Data.Spec.Template, m.client,
		m.Log, m.Data.Labels[clusterv1.ClusterNameLabel],
	)
	if err != nil {
		return err
	}
	if m3dt == nil {
		return nil
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched Metal3DataTemplate")

	return m.releaseAddressesFromPool(ctx, *m3dt)
}

// addressFromPool contains the elements coming from an IPPool.
type addressFromPool struct {
	Address    ipamv1.IPAddressStr
	Prefix     int
	Gateway    ipamv1.IPAddressStr
	dnsServers []ipamv1.IPAddressStr
}

type reconciledClaim struct {
	claim      *capipamv1.IPAddressClaim
	m3Claim    *ipamv1.IPClaim
	fetchAgain bool
}

// getAddressesFromPool allocates IP addresses from all IP pools referenced by a [Metal3DataTemplate].
// It does so by creating IP claims for each referenced pool. It will check whether the claim was fulfilled
// and return a map containing all pools and addresses. If some claims are not fulfilled yet, it will
// return a Transient type ReconcileError, indicating that some addresses were not fully allocated yet.
func (m *DataManager) getAddressesFromPool(ctx context.Context,
	m3dt infrav1.Metal3DataTemplate,
	m3m *infrav1.Metal3Machine,
	machine *clusterv1.Machine,
	bmh *bmov1alpha1.BareMetalHost,
) (map[string]addressFromPool, error) {
	var err error
	addresses := map[string]addressFromPool{}

	poolRefs, err := getReferencedPools(m3dt, m3m, machine, bmh)
	if err != nil {
		return addresses, err
	}
	claims := map[string]reconciledClaim{}

	for pool, ref := range poolRefs {
		var rc reconciledClaim
		if isMetal3IPPoolRef(ref) {
			rc, err = m.ensureM3IPClaim(ctx, ref)
		} else {
			rc, err = m.ensureIPClaim(ctx, ref)
		}
		if err != nil {
			return addresses, err
		}
		claims[pool] = rc
	}

	requeue := false
	for pool, ref := range poolRefs {
		rc, ok := claims[pool]
		if !ok {
			continue
		}
		if rc.fetchAgain {
			if rc.m3Claim != nil {
				rc.m3Claim, err = fetchM3IPClaim(ctx, m.client, m.Log, m.Data.Name+"-"+ref.Name, m.Data.Namespace)
			} else {
				err = m.client.Get(ctx, types.NamespacedName{Namespace: m.Data.Namespace, Name: m.Data.Name + "-" + ref.Name}, rc.claim)
			}
			if err != nil {
				// We ignore erros here. If they are persistent they will be handled during the next reconciliation.
				continue
			}
		}
		m.Log.Info("Allocating address from IPPool", "pool name", pool)
		var itemRequeue bool
		if rc.m3Claim != nil {
			addresses[pool], itemRequeue, err = m.addressFromM3Claim(ctx, ref, rc.m3Claim)
		} else if rc.claim != nil {
			addresses[pool], itemRequeue, err = m.addressFromClaim(ctx, ref, rc.claim)
		}
		requeue = requeue || itemRequeue
		if err != nil {
			return addresses, err
		}
	}

	m.Log.Info("done allocating addresses", "addresses", addresses, "requeue", requeue)
	if requeue {
		return addresses, WithTransientError(nil, requeueAfter)
	}
	return addresses, nil
}

func isMetal3IPPoolRef(ref corev1.TypedLocalObjectReference) bool {
	return (ref.APIGroup != nil && *ref.APIGroup == "ipam.metal3.io" && ref.Kind == IPPoolKind) ||
		((ref.APIGroup == nil || *ref.APIGroup == "") && ref.Kind == "")
}

// releaseAddressesFromPool releases all addresses allocated by a [Metal3DataTemplate] by deleting the IP claims.
func (m *DataManager) releaseAddressesFromPool(ctx context.Context, m3dt infrav1.Metal3DataTemplate) error {
	poolRefs, err := getReferencedPools(m3dt, nil, nil, nil)
	if err != nil {
		return err
	}
	for pool, ref := range poolRefs {
		m.Log.Info("Releasing address from IPPool", "pool name", pool)
		var err error
		if isMetal3IPPoolRef(ref) {
			err = m.releaseAddressFromM3Pool(ctx, ref)
		} else {
			err = m.releaseAddressFromPool(ctx, ref)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// poolRefs is used to consolidate the various references of a Metal3DataTemplate into a single map of references.
// The names of the referenced pools need to be unique.
type poolRefs map[string]corev1.TypedLocalObjectReference

// addRef adds a reference to the map.
// It defaults the reference's group and kind to metal3 pools.
// It returns an error if a reference with the same name but a different group or kind already exists.
func (p poolRefs) addRef(ref corev1.TypedLocalObjectReference) error {
	if ref.APIGroup == nil || *ref.APIGroup == "" {
		ref.APIGroup = ptr.To("ipam.metal3.io")
	}
	if ref.Kind == "" {
		ref.Kind = IPPoolKind
	}

	old, exists := p[ref.Name]
	if !exists {
		p[ref.Name] = ref
		return nil
	}

	if *old.APIGroup != *ref.APIGroup || old.Kind != ref.Kind {
		return errors.New("multiple references with the same name but different resource types")
	}

	return nil
}

// addFromPool adds a pool reference from a [FromPool] value.
func (p poolRefs) addFromPool(pool infrav1.FromPool) error {
	return p.addRef(corev1.TypedLocalObjectReference{Name: pool.Name, APIGroup: ptr.To(pool.APIGroup), Kind: pool.Kind})
}

// addName adds a reference to a metal3 pool using just its name.
func (p poolRefs) addName(name string) error {
	if name == "" {
		return nil
	}
	return p.addRef(corev1.TypedLocalObjectReference{Name: name})
}

// addFromAnnotation resolves a pool reference from an annotation and adds it to the pool refs.
// The annotation value should be a string containing the pool name.
// If the annotation pointer is nil or objects are nil (e.g., during release), this function returns nil without error.
func (p poolRefs) addFromAnnotation(
	fromPoolAnnotation *infrav1.FromPoolAnnotation,
	m3m *infrav1.Metal3Machine,
	machine *clusterv1.Machine,
	bmh *bmov1alpha1.BareMetalHost,
) error {
	if fromPoolAnnotation == nil {
		return nil
	}

	if m3m == nil && machine == nil && bmh == nil {
		return nil
	}

	annotationValue, err := getValueFromAnnotation(fromPoolAnnotation.Object, fromPoolAnnotation.Annotation, m3m, machine, bmh)
	if err != nil {
		return err
	}

	if annotationValue == "" {
		return fmt.Errorf("annotation %s not found or empty on %s", fromPoolAnnotation.Annotation, fromPoolAnnotation.Object)
	}

	ref := corev1.TypedLocalObjectReference{
		Name:     annotationValue,
		APIGroup: ptr.To("ipam.metal3.io"),
		Kind:     IPPoolKind,
	}

	return p.addRef(ref)
}

// getReferencedPools returns a map containing references to all pools mentioned by a [Metal3DataTemplate].
// It resolves both direct pool references and annotation-based references.
func getReferencedPools(m3dt infrav1.Metal3DataTemplate,
	m3m *infrav1.Metal3Machine,
	machine *clusterv1.Machine,
	bmh *bmov1alpha1.BareMetalHost,
) (map[string]corev1.TypedLocalObjectReference, error) {
	pools := poolRefs{}
	if m3dt.Spec.MetaData != nil {
		for _, pool := range m3dt.Spec.MetaData.IPAddressesFromPool {
			if err := pools.addFromPool(pool); err != nil {
				return pools, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.PrefixesFromPool {
			if err := pools.addFromPool(pool); err != nil {
				return pools, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.GatewaysFromPool {
			if err := pools.addFromPool(pool); err != nil {
				return pools, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.DNSServersFromPool {
			if err := pools.addFromPool(pool); err != nil {
				return pools, err
			}
		}
	}
	if m3dt.Spec.NetworkData != nil {
		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4 { //nolint:dupl
			if err := pools.addFromAnnotation(network.FromPoolAnnotation, m3m, machine, bmh); err != nil {
				return pools, err
			} else if network.FromPoolRef != nil && network.FromPoolRef.Name != "" {
				if err := pools.addRef(*network.FromPoolRef); err != nil {
					return pools, err
				}
			} else if network.IPAddressFromIPPool != "" {
				if err := pools.addName(network.IPAddressFromIPPool); err != nil {
					return pools, err
				}
			}

			for _, route := range network.Routes {
				if err := pools.addFromAnnotation(route.Gateway.FromPoolAnnotation, m3m, machine, bmh); err != nil {
					return pools, err
				} else if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
					if err := pools.addRef(*route.Gateway.FromPoolRef); err != nil {
						return pools, err
					}
				} else if route.Gateway.FromIPPool != nil {
					if err := pools.addName(*route.Gateway.FromIPPool); err != nil {
						return pools, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					if err := pools.addName(*route.Services.DNSFromIPPool); err != nil {
						return pools, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6 { //nolint:dupl
			if err := pools.addFromAnnotation(network.FromPoolAnnotation, m3m, machine, bmh); err != nil {
				return pools, err
			} else if network.FromPoolRef != nil && network.FromPoolRef.Name != "" {
				if err := pools.addRef(*network.FromPoolRef); err != nil {
					return pools, err
				}
			} else if network.IPAddressFromIPPool != "" {
				if err := pools.addName(network.IPAddressFromIPPool); err != nil {
					return pools, err
				}
			}
			for _, route := range network.Routes {
				if err := pools.addFromAnnotation(route.Gateway.FromPoolAnnotation, m3m, machine, bmh); err != nil {
					return pools, err
				} else if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
					if err := pools.addRef(*route.Gateway.FromPoolRef); err != nil {
						return pools, err
					}
				} else if route.Gateway.FromIPPool != nil {
					if err := pools.addName(*route.Gateway.FromIPPool); err != nil {
						return pools, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					if err := pools.addName(*route.Services.DNSFromIPPool); err != nil {
						return pools, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
					if err := pools.addRef(*route.Gateway.FromPoolRef); err != nil {
						return pools, err
					}
				} else if route.Gateway.FromIPPool != nil {
					if err := pools.addName(*route.Gateway.FromIPPool); err != nil {
						return pools, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					if err := pools.addName(*route.Services.DNSFromIPPool); err != nil {
						return pools, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
					if err := pools.addRef(*route.Gateway.FromPoolRef); err != nil {
						return pools, err
					}
				} else if route.Gateway.FromIPPool != nil {
					if err := pools.addName(*route.Gateway.FromIPPool); err != nil {
						return pools, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					if err := pools.addName(*route.Services.DNSFromIPPool); err != nil {
						return pools, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6SLAAC {
			for _, route := range network.Routes {
				if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
					if err := pools.addRef(*route.Gateway.FromPoolRef); err != nil {
						return pools, err
					}
				} else if route.Gateway.FromIPPool != nil {
					if err := pools.addName(*route.Gateway.FromIPPool); err != nil {
						return pools, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					if err := pools.addName(*route.Services.DNSFromIPPool); err != nil {
						return pools, err
					}
				}
			}
		}
		if m3dt.Spec.NetworkData.Services.DNSFromIPPool != nil {
			if err := pools.addName(*m3dt.Spec.NetworkData.Services.DNSFromIPPool); err != nil {
				return pools, err
			}
		}
	}
	return pools, nil
}

// m3IPClaimObjectMeta always returns ObjectMeta with Data labels, additional labels (DataLabelName/PoolLabelName)
// will be added to Data labels in case preallocation is enabled.
func (m *DataManager) m3IPClaimObjectMeta(name, poolRefName string, preallocationEnabled bool) *metav1.ObjectMeta {
	if preallocationEnabled {
		if m.Data.Labels == nil {
			m.Data.Labels = map[string]string{}
		}
		m.Data.Labels[DataLabelName] = m.Data.Name
		m.Data.Labels[PoolLabelName] = poolRefName
	}
	return &metav1.ObjectMeta{
		Name:       name + "-" + poolRefName,
		Namespace:  m.Data.Namespace,
		Finalizers: []string{infrav1.DataFinalizer},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: m.Data.APIVersion,
				Kind:       m.Data.Kind,
				Name:       m.Data.Name,
				UID:        m.Data.UID,
				Controller: ptr.To(true),
			},
		},
		Labels: m.Data.Labels,
	}
}

// ensureM3IPClaim ensures that a claim for a referenced pool exists.
// It returns the claim and whether to fetch the claim again when fetching IP addresses.
func (m *DataManager) ensureM3IPClaim(ctx context.Context, poolRef corev1.TypedLocalObjectReference) (reconciledClaim, error) {
	m.Log.Info("Ensuring Metal3IPClaim for Metal3Data", "Metal3Data", m.Data.Name)
	ipClaim, err := fetchM3IPClaim(ctx, m.client, m.Log, m.Data.Name+"-"+poolRef.Name, m.Data.Namespace)
	if err == nil {
		return reconciledClaim{m3Claim: ipClaim}, nil
	}

	var reconcileError ReconcileError
	if !errors.As(err, &reconcileError) {
		return reconciledClaim{m3Claim: ipClaim}, err
	}

	m3dt, err := fetchM3DataTemplate(ctx, &m.Data.Spec.Template, m.client,
		m.Log, m.Data.Labels[clusterv1.ClusterNameLabel],
	)
	if err != nil {
		return reconciledClaim{m3Claim: ipClaim}, err
	}
	if m3dt == nil {
		return reconciledClaim{m3Claim: ipClaim}, nil
	}
	m.Log.Info("Fetched Metal3DataTemplate", "Metal3DataTemplate", m3dt.Name)

	// Fetch the Metal3Machine, to get the related info
	m3m, err := m.getM3Machine(ctx, m3dt)
	if err != nil {
		return reconciledClaim{m3Claim: ipClaim}, err
	}
	if m3m == nil {
		return reconciledClaim{m3Claim: ipClaim}, nil
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched Metal3Machine", "Metal3Machine", m3m.Name)

	// Fetch the BMH associated with the M3M
	bmh, err := getHost(ctx, m3m, m.client, m.Log)
	if err != nil {
		return reconciledClaim{m3Claim: ipClaim}, err
	}
	if bmh == nil {
		return reconciledClaim{m3Claim: ipClaim}, WithTransientError(errors.New("no associated BMH yet"), requeueAfter)
	}
	m.Log.V(VerbosityLevelDebug).Info("Fetched BMH", "BMH", bmh.Name)

	ipClaim, err = fetchM3IPClaim(ctx, m.client, m.Log, bmh.Name+"-"+poolRef.Name, m.Data.Namespace)
	if err == nil {
		return reconciledClaim{m3Claim: ipClaim}, nil
	}
	if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
		return reconciledClaim{m3Claim: ipClaim}, err
	}

	m.Log.Info("Creating Metal3IPClaim")
	var ObjMeta *metav1.ObjectMeta
	if EnableBMHNameBasedPreallocation {
		// if EnableBMHNameBasedPreallocation enabled, name of the m3IPClaim is based on the BMH name
		ObjMeta = m.m3IPClaimObjectMeta(bmh.Name, poolRef.Name, true)
	} else {
		// otherwise, name of the m3IPClaim is based on the m3Data name
		ObjMeta = m.m3IPClaimObjectMeta(m.Data.Name, poolRef.Name, false)
	}
	// Create the claim
	ipClaim = &ipamv1.IPClaim{
		ObjectMeta: *ObjMeta,
		Spec: ipamv1.IPClaimSpec{
			Pool: corev1.ObjectReference{
				Name:      poolRef.Name,
				Namespace: m.Data.Namespace,
			},
		},
	}
	err = createObject(ctx, m.client, ipClaim)
	if err != nil {
		if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
			return reconciledClaim{m3Claim: ipClaim}, err
		}
	}

	m.Log.Info("Metal3IPClaim created successfully", "Metal3IPClaim", ipClaim.Name)
	return reconciledClaim{m3Claim: ipClaim, fetchAgain: true}, nil
}

// addressFromM3Claim retrieves the [Metal3IPAddress] for a [Metal3IPClaim].
func (m *DataManager) addressFromM3Claim(ctx context.Context, poolRef corev1.TypedLocalObjectReference, ipClaim *ipamv1.IPClaim) (addressFromPool, bool, error) {
	if ipClaim == nil {
		return addressFromPool{}, true, errors.New("no claim provided")
	}

	// Is it "our" ipClaim, or does it belong to an old and deleted Metal3Data with the same name?
	matchingOwnerRef := false
	for _, ownerRef := range ipClaim.OwnerReferences {
		if ownerRef.UID == m.Data.GetUID() {
			matchingOwnerRef = true
		}
	}

	if !ipClaim.DeletionTimestamp.IsZero() {
		if !matchingOwnerRef {
			// It is not our IPClaim so we should not use it. Attempt to remove finalizer if it is still there.
			m.Log.Info("Found old IPClaim with deletion timestamp. Attempting to clean up and requeue.", "IPClaim", ipClaim)
			if controllerutil.ContainsFinalizer(ipClaim, infrav1.DataFinalizer) {
				controllerutil.RemoveFinalizer(ipClaim, infrav1.DataFinalizer)
				err := updateObject(ctx, m.client, ipClaim)
				if err != nil {
					m.Log.Info("Failed to remove finalizer from old IPClaim", "IPClaim", ipClaim, "error", err)
				}
			}
			return addressFromPool{}, true, nil
		}
		m.Log.Info("IPClaim has deletion timestamp but is still in use!", "IPClaim", ipClaim)
	} else if !matchingOwnerRef {
		// It is not our IPClaim, but it does not appear to be deleting either.
		// This could happen due to misconfiguration (nameclash) or because the IPClaim
		// just didn't get the deletionTimestamp before the new Metal3Data was created (race condition).
		m.Log.Info("Found IPClaim with same name but different UID. Requeing and hoping it will go away.", "IPClaim", ipClaim)
		return addressFromPool{}, true, nil
	}

	if ipClaim.Status.ErrorMessage != nil {
		m.setError(ctx, fmt.Sprintf(
			"IP Allocation for %v failed : %v", poolRef.Name, *ipClaim.Status.ErrorMessage,
		))
		return addressFromPool{}, false, errors.New(*m.Data.Status.ErrorMessage)
	}

	// verify if allocation is there, if not requeue
	if ipClaim.Status.Address == nil {
		return addressFromPool{}, true, nil
	}

	// get Metal3IPAddress object
	ipAddress := &ipamv1.IPAddress{}
	addressNamespacedName := types.NamespacedName{
		Name:      ipClaim.Status.Address.Name,
		Namespace: m.Data.Namespace,
	}

	if err := m.client.Get(ctx, addressNamespacedName, ipAddress); err != nil {
		if apierrors.IsNotFound(err) {
			m.Log.Info("IPAddress not found, requeuing", "IPAddress", ipClaim.Status.Address.Name)
			return addressFromPool{}, true, nil
		}
		m.Log.Error(err, "Unable to get IPAddress.", "IPAddress", ipClaim.Status.Address.Name)
		return addressFromPool{}, false, err
	}

	gateway := ipamv1.IPAddressStr("")
	if ipAddress.Spec.Gateway != nil {
		gateway = *ipAddress.Spec.Gateway
	}

	return addressFromPool{
		Address:    ipAddress.Spec.Address,
		Prefix:     ipAddress.Spec.Prefix,
		Gateway:    gateway,
		dnsServers: ipAddress.Spec.DNSServers,
	}, false, nil
}

// releaseAddressFromM3Pool deletes the Metal3IPClaim for a referenced pool.
func (m *DataManager) releaseAddressFromM3Pool(ctx context.Context, poolRef corev1.TypedLocalObjectReference) error {
	var ipClaim *ipamv1.IPClaim
	var err, finalizerErr error
	ipClaimsList, err := m.fetchIPClaimsWithLabels(ctx, poolRef.Name)
	if err != nil {
		m.Log.Error(err, "Failed to fetch IPClaims with labels", "poolRef", poolRef, "Metal3Data", m.Data.Name)
	}
	if len(ipClaimsList) > 0 {
		for _, ipClaimWithLabels := range ipClaimsList {
			// remove finalizers from Metal3IPClaim first before proceeding to deletion in case
			// EnableBMHNameBasedPreallocation is set to True.
			if controllerutil.RemoveFinalizer(&ipClaimWithLabels, infrav1.DataFinalizer) {
				finalizerErr = m.client.Update(ctx, &ipClaimWithLabels)
				if finalizerErr != nil {
					return finalizerErr
				}
			}
			err = deleteObject(ctx, m.client, &ipClaimWithLabels)
			if err != nil {
				return err
			}
		}
	}
	ipClaim, err = fetchM3IPClaim(ctx, m.client, m.Log, m.Data.Name+"-"+poolRef.Name, m.Data.Namespace)
	if err != nil {
		var reconcileError ReconcileError
		if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
			return err
		}
		return nil
	}

	// remove finalizers from Metal3IPClaim before proceeding to Metal3IPClaim deletion.
	if controllerutil.RemoveFinalizer(ipClaim, infrav1.DataFinalizer) {
		finalizerErr = m.client.Update(ctx, ipClaim)
		if finalizerErr != nil {
			return finalizerErr
		}
	}

	// delete Metal3IPClaim object.
	return deleteObject(ctx, m.client, ipClaim)
}

// ensureIPClaim creates a CAPI IPAddressClaim for a pool if it does not exist yet.
func (m *DataManager) ensureIPClaim(ctx context.Context, poolRef corev1.TypedLocalObjectReference) (reconciledClaim, error) {
	claim := &capipamv1.IPAddressClaim{}
	nn := types.NamespacedName{
		Namespace: m.Data.Namespace,
		Name:      m.Data.Name + "-" + poolRef.Name,
	}
	if err := m.client.Get(ctx, nn, claim); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciledClaim{claim: claim}, err
		}
	}
	if claim.Name != "" {
		return reconciledClaim{claim: claim}, nil
	}

	// No claim exists, we create a new one
	claim = &capipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Data.Name + "-" + poolRef.Name,
			Namespace: m.Data.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: m.Data.APIVersion,
					Kind:       m.Data.Kind,
					Name:       m.Data.Name,
					UID:        m.Data.UID,
					Controller: ptr.To(true),
				},
			},
			Labels: m.Data.Labels,
			Finalizers: []string{
				infrav1.DataFinalizer,
			},
		},
		Spec: capipamv1.IPAddressClaimSpec{
			PoolRef: ConvertTypedLocalObjectReferenceToIPPoolReference(poolRef),
		},
	}

	err := m.client.Create(ctx, claim)
	// if the claim already exists we can try to fetch it again
	if err == nil || apierrors.IsAlreadyExists(err) {
		return reconciledClaim{claim: claim, fetchAgain: true}, nil
	}
	return reconciledClaim{claim: claim}, err
}

// addressFromClaim retrieves the IPAddress for a CAPI IPAddressClaim.
func (m *DataManager) addressFromClaim(ctx context.Context, _ corev1.TypedLocalObjectReference, claim *capipamv1.IPAddressClaim) (addressFromPool, bool, error) {
	if claim == nil {
		return addressFromPool{}, true, errors.New("no claim provided")
	}
	if !claim.DeletionTimestamp.IsZero() {
		// This IPClaim is about to be deleted so we cannot use it. Requeue.
		m.Log.Info("Found IPClaim with deletion timestamp, requeuing.", "IPClaim", claim)
		return addressFromPool{}, true, nil
	}

	if claim.Status.AddressRef.Name == "" {
		return addressFromPool{}, true, nil
	}

	address := &capipamv1.IPAddress{}
	addressNamespacedName := types.NamespacedName{
		Name:      claim.Status.AddressRef.Name,
		Namespace: m.Data.Namespace,
	}

	if err := m.client.Get(ctx, addressNamespacedName, address); err != nil {
		if apierrors.IsNotFound(err) {
			return addressFromPool{}, true, nil
		}
		return addressFromPool{}, false, err
	}

	a := addressFromPool{
		Address:    ipamv1.IPAddressStr(address.Spec.Address),
		Prefix:     int(*address.Spec.Prefix),
		Gateway:    ipamv1.IPAddressStr(address.Spec.Gateway),
		dnsServers: []ipamv1.IPAddressStr{},
	}
	return a, false, nil
}

// releaseAddressFromPool deletes the CAPI IP claim for a pool.
func (m *DataManager) releaseAddressFromPool(ctx context.Context, poolRef corev1.TypedLocalObjectReference) error {
	claim := &capipamv1.IPAddressClaim{}
	nn := types.NamespacedName{
		Namespace: m.Data.Namespace,
		Name:      m.Data.Name + "-" + poolRef.Name,
	}
	if err := m.client.Get(ctx, nn, claim); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if controllerutil.RemoveFinalizer(claim, infrav1.DataFinalizer) {
		if err := m.client.Update(ctx, claim); err != nil {
			return err
		}
	}

	return deleteObject(ctx, m.client, claim)
}

// renderNetworkData renders the networkData into an object that will be
// marshalled into the secret.
func renderNetworkData(m3dt *infrav1.Metal3DataTemplate,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost,
	poolAddresses map[string]addressFromPool,
) ([]byte, error) {
	if m3dt.Spec.NetworkData == nil {
		return nil, nil
	}
	var err error

	networkData := map[string][]any{}

	networkData["links"], err = renderNetworkLinks(m3dt.Spec.NetworkData.Links, m3m, machine, bmh)
	if err != nil {
		return nil, err
	}

	networkData["networks"], err = renderNetworkNetworks(m3dt.Spec.NetworkData.Networks, poolAddresses, m3m, machine, bmh)
	if err != nil {
		return nil, err
	}

	networkData["services"], err = renderNetworkServices(m3dt.Spec.NetworkData.Services, poolAddresses)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(networkData)
}

// renderNetworkServices renders the services.
func renderNetworkServices(services infrav1.NetworkDataService, poolAddresses map[string]addressFromPool) ([]any, error) {
	data := []any{}

	for _, service := range services.DNS {
		data = append(data, map[string]any{
			"type":    "dns",
			"address": service,
		})
	}

	if services.DNSFromIPPool != nil {
		poolAddress, ok := poolAddresses[*services.DNSFromIPPool]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		for _, service := range poolAddress.dnsServers {
			data = append(data, map[string]any{
				"type":    "dns",
				"address": service,
			})
		}
	}

	return data, nil
}

// renderNetworkLinks renders the different types of links.
func renderNetworkLinks(networkLinks infrav1.NetworkDataLink,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost) ([]any, error) {
	data := []any{}

	// Bond links
	for _, link := range networkLinks.Bonds {
		macAddress, err := getLinkMacAddress(link.MACAddress, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{
			"type":                  "bond",
			"id":                    link.Id,
			"mtu":                   link.MTU,
			"ethernet_mac_address":  macAddress,
			"bond_mode":             link.BondMode,
			"bond_xmit_hash_policy": link.BondXmitHashPolicy,
			"bond_links":            link.BondLinks,
		}

		illegalParameters := []string{}
		for opt, value := range link.Parameters {
			target := "bond_" + opt
			_, ok := entry[target]
			if ok {
				illegalParameters = append(illegalParameters, opt)
				continue // Do not overwrite structured parameters
			}
			entry[target] = value
		}

		if 0 < len(illegalParameters) {
			return nil, fmt.Errorf("illegal params \"%s\" into bond params blob into link id=%s",
				strings.Join(illegalParameters, `", "`), link.Id)
		}

		data = append(data, entry)
	}

	// Ethernet links
	for _, link := range networkLinks.Ethernets {
		macAddress, err := getLinkMacAddress(link.MACAddress, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":                 link.Type,
			"id":                   link.Id,
			"mtu":                  link.MTU,
			"ethernet_mac_address": macAddress,
		})
	}

	// Vlan links
	for _, link := range networkLinks.Vlans {
		macAddress, err := getLinkMacAddress(link.MACAddress, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":             "vlan",
			"id":               link.Id,
			"mtu":              link.MTU,
			"vlan_mac_address": macAddress,
			"vlan_id":          link.VlanID,
			"vlan_link":        link.VlanLink,
		})
	}

	return data, nil
}

// renderNetworkNetworks renders the different types of network.
func renderNetworkNetworks(networks infrav1.NetworkDataNetwork,
	poolAddresses map[string]addressFromPool,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost,
) ([]any, error) {
	data := []any{}

	// IPv4 networks static allocation
	//nolint:dupl
	for _, network := range networks.IPv4 {
		var poolAddress addressFromPool
		var ok bool
		if network.FromPoolAnnotation != nil {
			poolName, err := getValueFromAnnotation(network.FromPoolAnnotation.Object, network.FromPoolAnnotation.Annotation, m3m, machine, bmh)
			if err != nil {
				return nil, err
			}
			poolAddress, ok = poolAddresses[poolName]
		} else if network.FromPoolRef != nil && network.FromPoolRef.Name != "" {
			poolAddress, ok = poolAddresses[network.FromPoolRef.Name]
		} else {
			poolAddress, ok = poolAddresses[network.IPAddressFromIPPool]
		}
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		ip := ipamv1.IPAddressv4Str(poolAddress.Address)
		mask := translateMask(poolAddress.Prefix, true)
		routes, err := getRoutesv4(network.Routes, poolAddresses, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":       "ipv4",
			"id":         network.ID,
			"link":       network.Link,
			"netmask":    mask,
			"ip_address": ip,
			"routes":     routes,
		})
	}

	// IPv6 networks static allocation
	//nolint:dupl
	for _, network := range networks.IPv6 {
		var poolAddress addressFromPool
		var ok bool
		if network.FromPoolAnnotation != nil {
			poolName, err := getValueFromAnnotation(network.FromPoolAnnotation.Object, network.FromPoolAnnotation.Annotation, m3m, machine, bmh)
			if err != nil {
				return nil, err
			}
			poolAddress, ok = poolAddresses[poolName]
		} else if network.FromPoolRef != nil && network.FromPoolRef.Name != "" {
			poolAddress, ok = poolAddresses[network.FromPoolRef.Name]
		} else {
			poolAddress, ok = poolAddresses[network.IPAddressFromIPPool]
		}
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		ip := ipamv1.IPAddressv6Str(poolAddress.Address)
		mask := translateMask(poolAddress.Prefix, false)
		routes, err := getRoutesv6(network.Routes, poolAddresses, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":       "ipv6",
			"id":         network.ID,
			"link":       network.Link,
			"netmask":    mask,
			"ip_address": ip,
			"routes":     routes,
		})
	}

	// IPv4 networks DHCP allocation
	for _, network := range networks.IPv4DHCP {
		routes, err := getRoutesv4(network.Routes, poolAddresses, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":   "ipv4_dhcp",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	// IPv6 networks DHCP allocation
	for _, network := range networks.IPv6DHCP {
		routes, err := getRoutesv6(network.Routes, poolAddresses, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":   "ipv6_dhcp",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	// IPv6 networks SLAAC allocation
	for _, network := range networks.IPv6SLAAC {
		routes, err := getRoutesv6(network.Routes, poolAddresses, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"type":   "ipv6_slaac",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	return data, nil
}

// getRoutesv4 returns the IPv4 routes.
// combining getRoutesv4 and getRoutesv6 would be possible with
// generics but it would make it difficult to understand
//
//nolint:dupl
func getRoutesv4(netRoutes []infrav1.NetworkDataRoutev4,
	poolAddresses map[string]addressFromPool,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost,
) ([]any, error) {
	routes := []any{}
	for _, route := range netRoutes {
		gateway := ipamv1.IPAddressv4Str("")
		if route.Gateway.String != nil {
			gateway = *route.Gateway.String
		} else if route.Gateway.FromPoolAnnotation != nil {
			poolName, err := getValueFromAnnotation(route.Gateway.FromPoolAnnotation.Object, route.Gateway.FromPoolAnnotation.Annotation, m3m, machine, bmh)
			if err != nil {
				return []any{}, err
			}
			poolAddress, ok := poolAddresses[poolName]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv4Str(poolAddress.Gateway)
		} else if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
			poolAddress, ok := poolAddresses[route.Gateway.FromPoolRef.Name]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv4Str(poolAddress.Gateway)
		} else if route.Gateway.FromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Gateway.FromIPPool]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv4Str(poolAddress.Gateway)
		}
		services := []any{}
		for _, service := range route.Services.DNS {
			services = append(services, map[string]any{
				"type":    "dns",
				"address": service,
			})
		}
		if route.Services.DNSFromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Services.DNSFromIPPool]
			if !ok {
				return []any{}, errors.New("Pool not found in cache")
			}
			for _, service := range poolAddress.dnsServers {
				services = append(services, map[string]any{
					"type":    "dns",
					"address": service,
				})
			}
		}
		mask := translateMask(route.Prefix, true)
		routes = append(routes, map[string]any{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  gateway,
			"services": services,
		})
	}
	return routes, nil
}

// getRoutesv6 returns the IPv6 routes.
//
//nolint:dupl
func getRoutesv6(netRoutes []infrav1.NetworkDataRoutev6,
	poolAddresses map[string]addressFromPool,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost,
) ([]any, error) {
	routes := []any{}
	for _, route := range netRoutes {
		gateway := ipamv1.IPAddressv6Str("")
		if route.Gateway.String != nil {
			gateway = *route.Gateway.String
		} else if route.Gateway.FromPoolAnnotation != nil {
			poolName, err := getValueFromAnnotation(route.Gateway.FromPoolAnnotation.Object, route.Gateway.FromPoolAnnotation.Annotation, m3m, machine, bmh)
			if err != nil {
				return []any{}, err
			}
			poolAddress, ok := poolAddresses[poolName]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv6Str(poolAddress.Gateway)
		} else if route.Gateway.FromPoolRef != nil && route.Gateway.FromPoolRef.Name != "" {
			poolAddress, ok := poolAddresses[route.Gateway.FromPoolRef.Name]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv6Str(poolAddress.Gateway)
		} else if route.Gateway.FromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Gateway.FromIPPool]
			if !ok {
				return []any{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv6Str(poolAddress.Gateway)
		}
		services := []any{}
		for _, service := range route.Services.DNS {
			services = append(services, map[string]any{
				"type":    "dns",
				"address": service,
			})
		}
		if route.Services.DNSFromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Services.DNSFromIPPool]
			if !ok {
				return []any{}, errors.New("Pool not found in cache")
			}
			for _, service := range poolAddress.dnsServers {
				services = append(services, map[string]any{
					"type":    "dns",
					"address": service,
				})
			}
		}
		mask := translateMask(route.Prefix, false)
		routes = append(routes, map[string]any{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  gateway,
			"services": services,
		})
	}
	return routes, nil
}

// translateMask transforms a mask given as integer into a dotted-notation string.
func translateMask(maskInt int, ipv4 bool) any {
	IPv4MaskLen := 32
	IPv6MaskLen := 128
	if ipv4 {
		// Get the mask by concatenating the IPv4 prefix of net package and the mask
		address := net.IP(append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255},
			[]byte(net.CIDRMask(maskInt, IPv4MaskLen))...,
		)).String()
		return ipamv1.IPAddressv4Str(address)
	}
	// get the mask
	address := net.IP(net.CIDRMask(maskInt, IPv6MaskLen)).String()
	return ipamv1.IPAddressv6Str(address)
}

// getLinkMacAddress returns the mac address.
func getLinkMacAddress(mac *infrav1.NetworkLinkEthernetMac,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost) (
	string, error,
) {
	var macaddress, err = "", errors.New("no MAC address given")

	if mac.String != nil {
		// if a string was given
		macaddress, err = *mac.String, nil
	} else if mac.FromHostInterface != nil {
		// if a host interface is given
		macaddress, err = getBMHMacByName(*mac.FromHostInterface, bmh)
	} else if mac.FromAnnotation != nil {
		// if an annotation reference is given
		macaddress, err = getValueFromAnnotation(mac.FromAnnotation.Object,
			mac.FromAnnotation.Annotation, m3m, machine, bmh)
	}

	if err != nil {
		return "", err
	}

	matching, err := regexp.MatchString("^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$", macaddress)
	if err != nil {
		return "", err
	}
	if !matching {
		return "", fmt.Errorf("bad MAC address: %s", macaddress)
	}

	return macaddress, nil
}

// renderMetaData renders the MetaData items.
func renderMetaData(m3d *infrav1.Metal3Data, m3dt *infrav1.Metal3DataTemplate,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost,
	poolAddresses map[string]addressFromPool,
) ([]byte, error) {
	if m3dt.Spec.MetaData == nil {
		return nil, nil
	}
	metadata := make(map[string]string)

	// Mac addresses
	for _, entry := range m3dt.Spec.MetaData.FromHostInterfaces {
		value, err := getBMHMacByName(entry.Interface, bmh)
		if err != nil {
			return nil, err
		}
		metadata[entry.Key] = value
	}

	// IP addresses
	for _, entry := range m3dt.Spec.MetaData.IPAddressesFromPool {
		poolAddress, ok := poolAddresses[entry.Name]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		metadata[entry.Key] = string(poolAddress.Address)
	}

	// Prefixes
	for _, entry := range m3dt.Spec.MetaData.PrefixesFromPool {
		poolAddress, ok := poolAddresses[entry.Name]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		metadata[entry.Key] = strconv.Itoa(poolAddress.Prefix)
	}

	// Gateways
	for _, entry := range m3dt.Spec.MetaData.GatewaysFromPool {
		poolAddress, ok := poolAddresses[entry.Name]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		metadata[entry.Key] = string(poolAddress.Gateway)
	}

	// Indexes
	for _, entry := range m3dt.Spec.MetaData.Indexes {
		if entry.Step == 0 {
			entry.Step = 1
		}
		metadata[entry.Key] = entry.Prefix + strconv.Itoa(entry.Offset+m3d.Spec.Index*entry.Step) + entry.Suffix
	}

	// Namespaces
	for _, entry := range m3dt.Spec.MetaData.Namespaces {
		metadata[entry.Key] = m3d.Namespace
	}

	// Object names
	for _, entry := range m3dt.Spec.MetaData.ObjectNames {
		switch strings.ToLower(entry.Object) {
		case m3machine:
			metadata[entry.Key] = m3m.Name
		case capimachine:
			metadata[entry.Key] = machine.Name
		case host:
			metadata[entry.Key] = bmh.Name
		default:
			return nil, errors.New("Unknown object type")
		}
	}

	// Labels
	for _, entry := range m3dt.Spec.MetaData.FromLabels {
		switch strings.ToLower(entry.Object) {
		case m3machine:
			metadata[entry.Key] = m3m.Labels[entry.Label]
		case capimachine:
			metadata[entry.Key] = machine.Labels[entry.Label]
		case host:
			metadata[entry.Key] = bmh.Labels[entry.Label]
		default:
			return nil, errors.New("Unknown object type")
		}
	}

	// Annotations
	for _, entry := range m3dt.Spec.MetaData.FromAnnotations {
		value, err := getValueFromAnnotation(entry.Object,
			entry.Annotation, m3m, machine, bmh)
		if err != nil {
			return nil, err
		}
		metadata[entry.Key] = value
	}

	// Strings
	for _, entry := range m3dt.Spec.MetaData.Strings {
		metadata[entry.Key] = entry.Value
	}
	providerid := fmt.Sprintf("%s/%s/%s", m3m.GetNamespace(), bmh.GetName(), m3m.GetName())
	metadata["providerid"] = providerid
	return yaml.Marshal(metadata)
}

// getBMHMacByName returns the mac address of the interface matching the name.
func getBMHMacByName(name string, bmh *bmov1alpha1.BareMetalHost) (string, error) {
	if bmh == nil || bmh.Status.HardwareDetails == nil || bmh.Status.HardwareDetails.NIC == nil {
		return "", errors.New("NICs list not populated")
	}
	for _, nics := range bmh.Status.HardwareDetails.NIC {
		if nics.Name == name {
			return nics.MAC, nil
		}
	}
	return "", fmt.Errorf("NIC name not found %v", name)
}

// getValueFromAnnotation returns an annotation from an object representing a machine.
func getValueFromAnnotation(object string, annotation string,
	m3m *infrav1.Metal3Machine, machine *clusterv1.Machine, bmh *bmov1alpha1.BareMetalHost) (string, error) {
	switch strings.ToLower(object) {
	case m3machine:
		if m3m == nil {
			return "", fmt.Errorf("%s is nil but referenced in annotation %s", object, annotation)
		}
		return m3m.Annotations[annotation], nil
	case capimachine:
		if machine == nil {
			return "", fmt.Errorf("%s is nil but referenced in annotation %s", object, annotation)
		}
		return machine.Annotations[annotation], nil
	case host:
		if bmh == nil {
			return "", fmt.Errorf("%s is nil but referenced in annotation %s", object, annotation)
		}
		return bmh.Annotations[annotation], nil
	default:
		return "", errors.New("Unknown object type")
	}
}

func (m *DataManager) getM3Machine(ctx context.Context, m3dt *infrav1.Metal3DataTemplate) (*infrav1.Metal3Machine, error) {
	if m.Data.Spec.Claim.Name == "" {
		return nil, errors.New("Metal3DataClaim name not set")
	}

	capm3DataClaim := &infrav1.Metal3DataClaim{}
	claimNamespacedName := types.NamespacedName{
		Name:      m.Data.Spec.Claim.Name,
		Namespace: m.Data.Namespace,
	}

	if err := m.client.Get(ctx, claimNamespacedName, capm3DataClaim); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, WithTransientError(nil, requeueAfter)
		}
		return nil, err
	}

	metal3MachineName := ""
	for _, ownerRef := range capm3DataClaim.OwnerReferences {
		oGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return nil, err
		}
		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if ownerRef.Kind == metal3MachineKind &&
			oGV.Group == infrav1.GroupVersion.Group {
			metal3MachineName = ownerRef.Name
			break
		}
	}
	if metal3MachineName == "" {
		return nil, errors.New("Metal3Machine not found in owner references")
	}

	return getM3Machine(ctx, m.client,
		m.Log, metal3MachineName, m.Data.Namespace, m3dt, true,
	)
}

// fetchM3IPClaim returns an IPClaim.
func fetchM3IPClaim(ctx context.Context, cl client.Client, mLog logr.Logger,
	name, namespace string,
) (*ipamv1.IPClaim, error) {
	// Fetch the Metal3Data
	metal3IPClaim := &ipamv1.IPClaim{}
	metal3ClaimName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := cl.Get(ctx, metal3ClaimName, metal3IPClaim); err != nil {
		if apierrors.IsNotFound(err) {
			errMessage := "Address claim not found"
			mLog.Info(errMessage)
			return nil, WithTransientError(errors.New(errMessage), requeueAfter)
		}
		err = errors.Wrap(err, "Failed to get address claim")
		return nil, err
	}
	return metal3IPClaim, nil
}

// fetchIPClaimsWithLabels returns a list of all IPClaims with matching labels.
func (m *DataManager) fetchIPClaimsWithLabels(ctx context.Context, pool string) ([]ipamv1.IPClaim, error) {
	allIPClaims := ipamv1.IPClaimList{}
	opts := []client.ListOption{
		&client.ListOptions{Namespace: m.Data.Namespace, Limit: DefaultListLimit},
		client.MatchingLabels{
			DataLabelName: m.Data.Name,
			PoolLabelName: pool,
		},
	}
	err := m.client.List(ctx, &allIPClaims, opts...)
	if err != nil {
		return nil, err
	}
	return allIPClaims.Items, nil
}
