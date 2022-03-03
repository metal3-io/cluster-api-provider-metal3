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
	"strconv"
	"strings"

	"github.com/go-logr/logr"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	m3machine   = "metal3machine"
	host        = "baremetalhost"
	capimachine = "machine"
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
	Data   *capm3.Metal3Data
	Log    logr.Logger
}

// NewDataManager returns a new helper for managing a Metal3Data object.
func NewDataManager(client client.Client,
	data *capm3.Metal3Data, dataLog logr.Logger) (*DataManager, error) {
	return &DataManager{
		client: client,
		Data:   data,
		Log:    dataLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *DataManager) SetFinalizer() {
	// If the Metal3Data doesn't have finalizer, add it.
	if !Contains(m.Data.Finalizers, capm3.DataFinalizer) {
		m.Data.Finalizers = append(m.Data.Finalizers,
			capm3.DataFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *DataManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.Data.Finalizers = Filter(m.Data.Finalizers,
		capm3.DataFinalizer,
	)
}

// clearError clears error message from Metal3Data status.
func (m *DataManager) clearError(ctx context.Context) {
	m.Data.Status.ErrorMessage = nil
}

// setError sets error message to Metal3Data status.
func (m *DataManager) setError(ctx context.Context, msg string) {
	m.Data.Status.ErrorMessage = &msg
}

// Reconcile handles Metal3Data events.
func (m *DataManager) Reconcile(ctx context.Context) error {
	m.clearError(ctx)

	if err := m.createSecrets(ctx); err != nil {
		if ok := errors.As(err, &hasRequeueAfterError); ok {
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
		m.Log, m.Data.Labels[clusterv1.ClusterLabelName],
	)
	if err != nil {
		return err
	}
	if m3dt == nil {
		return nil
	}
	m.Log.Info("Fetched Metal3DataTemplate")

	// Fetch the Metal3Machine, to get the related info
	m3m, err := m.getM3Machine(ctx, m3dt)
	if err != nil {
		return err
	}
	if m3m == nil {
		return errors.New("Unexpected error getching Metal3Machine")
	}
	m.Log.Info("Fetched Metal3Machine")

	// If the MetaData is given as part of Metal3DataTemplate
	if m3dt.Spec.MetaData != nil {
		// If the secret name is unset, set it
		if m.Data.Spec.MetaData == nil || m.Data.Spec.MetaData.Name == "" {
			m.Data.Spec.MetaData = &corev1.SecretReference{
				Name:      m3m.Name + "-metadata",
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
		// If the secret name is unset, set it
		if m.Data.Spec.NetworkData == nil || m.Data.Spec.NetworkData.Name == "" {
			m.Data.Spec.NetworkData = &corev1.SecretReference{
				Name:      m3m.Name + "-networkdata",
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

	// Fetch all the Metal3IPPools and set the OwnerReference. Check if the
	// IP address has been allocated, if so, fetch the address, gateway and prefix.
	poolAddresses, err := m.getAddressesFromPool(ctx, *m3dt)
	if err != nil {
		return err
	}

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, m.client, m3m.ObjectMeta)

	if err != nil {
		return errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		m.Log.Info("Waiting for Machine Controller to set OwnerRef on Metal3Machine")
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	m.Log.Info("Fetched Machine")

	// Fetch the BMH associated with the M3M
	bmh, err := getHost(ctx, m3m, m.client, m.Log)
	if err != nil {
		return err
	}
	if bmh == nil {
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	m.Log.Info("Fetched BMH")

	// Create the owner Ref for the secret
	ownerRefs := []metav1.OwnerReference{
		{
			Controller: pointer.BoolPtr(true),
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
			m.Data.Namespace, m3dt.Labels[clusterv1.ClusterLabelName],
			ownerRefs, map[string][]byte{"metaData": metadata},
		); err != nil {
			return err
		}
	}

	// The NetworkData secret must be created
	if apierrors.IsNotFound(networkDataErr) {
		m.Log.Info("Creating Networkdata secret")
		networkData, err := renderNetworkData(m.Data, m3dt, bmh, poolAddresses)
		if err != nil {
			return err
		}
		if err := createSecret(ctx, m.client, m.Data.Spec.NetworkData.Name,
			m.Data.Namespace, m3dt.Labels[clusterv1.ClusterLabelName],
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
		m.Log, m.Data.Labels[clusterv1.ClusterLabelName],
	)
	if err != nil {
		return err
	}
	if m3dt == nil {
		return nil
	}
	m.Log.Info("Fetched Metal3DataTemplate")

	return m.releaseAddressesFromPool(ctx, *m3dt)
}

// addressFromPool contains the elements coming from an IPPool.
type addressFromPool struct {
	address    ipamv1.IPAddressStr
	prefix     int
	gateway    ipamv1.IPAddressStr
	dnsServers []ipamv1.IPAddressStr
}

// getAddressesFromPool will fetch each Metal3IPPool referenced at least once,
// set the Ownerreference if not set, and check if the Metal3IPAddress has been
// allocated. If not, it will requeue once all IPPools were fetched. If all have
// been allocated it will return a map containing the IPPool name and the address,
// prefix and gateway from that IPPool.
func (m *DataManager) getAddressesFromPool(ctx context.Context,
	m3dt capm3.Metal3DataTemplate,
) (map[string]addressFromPool, error) {
	var err error
	requeue := false
	itemRequeue := false
	addresses := make(map[string]addressFromPool)
	if m3dt.Spec.MetaData != nil {
		for _, pool := range m3dt.Spec.MetaData.IPAddressesFromPool {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.PrefixesFromPool {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.GatewaysFromPool {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.DNSServersFromPool {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
		}
	}
	if m3dt.Spec.NetworkData != nil {
		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4 {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, network.IPAddressFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
			// network.IPAddressFromIPPool
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6 {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, network.IPAddressFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6SLAAC {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.getAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return addresses, err
					}
				}
			}
		}
		if m3dt.Spec.NetworkData.Services.DNSFromIPPool != nil {
			addresses, itemRequeue, err = m.getAddressFromPool(ctx, *m3dt.Spec.NetworkData.Services.DNSFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return addresses, err
			}
		}
	}
	if requeue {
		return addresses, &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	return addresses, nil
}

// releaseAddressesFromPool removes the OwnerReference on the IPPool objects.
func (m *DataManager) releaseAddressesFromPool(ctx context.Context,
	m3dt capm3.Metal3DataTemplate,
) error {
	var err error
	requeue := false
	itemRequeue := false
	addresses := make(map[string]bool)
	if m3dt.Spec.MetaData != nil {
		for _, pool := range m3dt.Spec.MetaData.IPAddressesFromPool {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			fmt.Println(requeue)
			if err != nil {
				return err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.PrefixesFromPool {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.GatewaysFromPool {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
		}
		for _, pool := range m3dt.Spec.MetaData.DNSServersFromPool {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, pool.Name, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
		}
	}
	if m3dt.Spec.NetworkData != nil {
		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4 {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, network.IPAddressFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
			// network.IPAddressFromIPPool
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6 {
			addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, network.IPAddressFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv4DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6DHCP {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
			}
		}

		for _, network := range m3dt.Spec.NetworkData.Networks.IPv6SLAAC {
			for _, route := range network.Routes {
				if route.Gateway.FromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Gateway.FromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
				if route.Services.DNSFromIPPool != nil {
					addresses, itemRequeue, err = m.releaseAddressFromPool(ctx, *route.Services.DNSFromIPPool, addresses)
					requeue = requeue || itemRequeue
					if err != nil {
						return err
					}
				}
			}
		}
		if m3dt.Spec.NetworkData.Services.DNSFromIPPool != nil {
			_, itemRequeue, err = m.releaseAddressFromPool(ctx, *m3dt.Spec.NetworkData.Services.DNSFromIPPool, addresses)
			requeue = requeue || itemRequeue
			if err != nil {
				return err
			}
		}
	}
	if requeue {
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	return nil
}

// getAddressFromPool adds an ownerReference on the referenced Metal3IPPool
// objects. It then tries to fetch the Metal3IPAddress if it exists, and asks
// for requeue if not ready.
func (m *DataManager) getAddressFromPool(ctx context.Context, poolName string,
	addresses map[string]addressFromPool,
) (map[string]addressFromPool, bool, error) {
	if addresses == nil {
		addresses = make(map[string]addressFromPool)
	}
	if entry, ok := addresses[poolName]; ok {
		if entry.address != "" {
			return addresses, false, nil
		}
	}
	addresses[poolName] = addressFromPool{}

	ipClaim, err := fetchM3IPClaim(ctx, m.client, m.Log, m.Data.Name+"-"+poolName,
		m.Data.Namespace,
	)
	if err != nil {
		if ok := errors.As(err, &hasRequeueAfterError); !ok {
			return addresses, false, err
		}
		// Create the claim
		ipClaim = &ipamv1.IPClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      m.Data.Name + "-" + poolName,
				Namespace: m.Data.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: m.Data.APIVersion,
						Kind:       m.Data.Kind,
						Name:       m.Data.Name,
						UID:        m.Data.UID,
						Controller: pointer.BoolPtr(true),
					},
				},
				Labels: m.Data.Labels,
			},
			Spec: ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      poolName,
					Namespace: m.Data.Namespace,
				},
			},
		}

		err = createObject(ctx, m.client, ipClaim)
		if err != nil {
			if ok := errors.As(err, &hasRequeueAfterError); !ok {
				return addresses, false, err
			}
		}
	}

	if ipClaim.Status.ErrorMessage != nil {
		m.Data.Status.ErrorMessage = pointer.StringPtr(fmt.Sprintf(
			"IP Allocation for %v failed : %v", poolName, ipClaim.Status.ErrorMessage,
		))
		return addresses, false, errors.New(*m.Data.Status.ErrorMessage)
	}

	// verify if allocation is there, if not requeue
	if ipClaim.Status.Address == nil {
		return addresses, true, nil
	}

	// get Metal3IPAddress object
	ipAddress := &ipamv1.IPAddress{}
	addressNamespacedName := types.NamespacedName{
		Name:      ipClaim.Status.Address.Name,
		Namespace: m.Data.Namespace,
	}

	if err := m.client.Get(ctx, addressNamespacedName, ipAddress); err != nil {
		if apierrors.IsNotFound(err) {
			return addresses, true, nil
		}
		return addresses, false, err
	}

	gateway := ipamv1.IPAddressStr("")
	if ipAddress.Spec.Gateway != nil {
		gateway = *ipAddress.Spec.Gateway
	}

	addresses[poolName] = addressFromPool{
		address:    ipAddress.Spec.Address,
		prefix:     ipAddress.Spec.Prefix,
		gateway:    gateway,
		dnsServers: ipAddress.Spec.DNSServers,
	}

	return addresses, false, nil
}

// releaseAddressFromPool removes the owner reference on existing referenced
// Metal3IPPool objects.
func (m *DataManager) releaseAddressFromPool(ctx context.Context, poolName string,
	addresses map[string]bool,
) (map[string]bool, bool, error) {
	if addresses == nil {
		addresses = make(map[string]bool)
	}
	if _, ok := addresses[poolName]; ok {
		return addresses, false, nil
	}
	addresses[poolName] = false

	ipClaim, err := fetchM3IPClaim(ctx, m.client, m.Log, m.Data.Name+"-"+poolName,
		m.Data.Namespace,
	)
	if err != nil {
		if ok := errors.As(err, &hasRequeueAfterError); !ok {
			return addresses, false, err
		}
		addresses[poolName] = true
		return addresses, false, nil
	}

	err = deleteObject(ctx, m.client, ipClaim)
	if err != nil {
		return addresses, false, err
	}

	addresses[poolName] = true
	return addresses, false, nil
}

// renderNetworkData renders the networkData into an object that will be
// marshalled into the secret.
func renderNetworkData(m3d *capm3.Metal3Data, m3dt *capm3.Metal3DataTemplate,
	bmh *bmo.BareMetalHost, poolAddresses map[string]addressFromPool,
) ([]byte, error) {
	if m3dt.Spec.NetworkData == nil {
		return nil, nil
	}
	var err error

	networkData := map[string][]interface{}{}

	networkData["links"], err = renderNetworkLinks(m3dt.Spec.NetworkData.Links, bmh)
	if err != nil {
		return nil, err
	}

	networkData["networks"], err = renderNetworkNetworks(
		m3dt.Spec.NetworkData.Networks, m3d, poolAddresses,
	)
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
func renderNetworkServices(services capm3.NetworkDataService, poolAddresses map[string]addressFromPool) ([]interface{}, error) {
	data := []interface{}{}

	for _, service := range services.DNS {
		data = append(data, map[string]interface{}{
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
			data = append(data, map[string]interface{}{
				"type":    "dns",
				"address": service,
			})
		}
	}

	return data, nil
}

// renderNetworkLinks renders the different types of links.
func renderNetworkLinks(networkLinks capm3.NetworkDataLink, bmh *bmo.BareMetalHost) ([]interface{}, error) {
	data := []interface{}{}

	// Ethernet links
	for _, link := range networkLinks.Ethernets {
		macAddress, err := getLinkMacAddress(link.MACAddress, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":                 link.Type,
			"id":                   link.Id,
			"mtu":                  link.MTU,
			"ethernet_mac_address": macAddress,
		})
	}

	// Bond links
	for _, link := range networkLinks.Bonds {
		macAddress, err := getLinkMacAddress(link.MACAddress, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":                 "bond",
			"id":                   link.Id,
			"mtu":                  link.MTU,
			"ethernet_mac_address": macAddress,
			"bond_mode":            link.BondMode,
			"bond_links":           link.BondLinks,
		})
	}

	// Vlan links
	for _, link := range networkLinks.Vlans {
		macAddress, err := getLinkMacAddress(link.MACAddress, bmh)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
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
func renderNetworkNetworks(networks capm3.NetworkDataNetwork,
	m3d *capm3.Metal3Data, poolAddresses map[string]addressFromPool,
) ([]interface{}, error) {
	data := []interface{}{}

	// IPv4 networks static allocation
	for _, network := range networks.IPv4 {
		poolAddress, ok := poolAddresses[network.IPAddressFromIPPool]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		ip := ipamv1.IPAddressv4Str(poolAddress.address)
		mask := translateMask(poolAddress.prefix, true)
		routes, err := getRoutesv4(network.Routes, poolAddresses)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":       "ipv4",
			"id":         network.ID,
			"link":       network.Link,
			"netmask":    mask,
			"ip_address": ip,
			"routes":     routes,
		})
	}

	// IPv6 networks static allocation
	for _, network := range networks.IPv6 {
		poolAddress, ok := poolAddresses[network.IPAddressFromIPPool]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		ip := ipamv1.IPAddressv6Str(poolAddress.address)
		mask := translateMask(poolAddress.prefix, false)
		routes, err := getRoutesv6(network.Routes, poolAddresses)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
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
		routes, err := getRoutesv4(network.Routes, poolAddresses)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":   "ipv4_dhcp",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	// IPv6 networks DHCP allocation
	for _, network := range networks.IPv6DHCP {
		routes, err := getRoutesv6(network.Routes, poolAddresses)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":   "ipv6_dhcp",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	// IPv6 networks SLAAC allocation
	for _, network := range networks.IPv6SLAAC {
		routes, err := getRoutesv6(network.Routes, poolAddresses)
		if err != nil {
			return nil, err
		}
		data = append(data, map[string]interface{}{
			"type":   "ipv6_slaac",
			"id":     network.ID,
			"link":   network.Link,
			"routes": routes,
		})
	}

	return data, nil
}

// getRoutesv4 returns the IPv4 routes.
func getRoutesv4(netRoutes []capm3.NetworkDataRoutev4,
	poolAddresses map[string]addressFromPool,
) ([]interface{}, error) {
	routes := []interface{}{}
	for _, route := range netRoutes {
		gateway := ipamv1.IPAddressv4Str("")
		if route.Gateway.String != nil {
			gateway = *route.Gateway.String
		} else if route.Gateway.FromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Gateway.FromIPPool]
			if !ok {
				return []interface{}{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv4Str(poolAddress.gateway)
		}
		services := []interface{}{}
		for _, service := range route.Services.DNS {
			services = append(services, map[string]interface{}{
				"type":    "dns",
				"address": service,
			})
		}
		if route.Services.DNSFromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Services.DNSFromIPPool]
			if !ok {
				return []interface{}{}, errors.New("Pool not found in cache")
			}
			for _, service := range poolAddress.dnsServers {
				services = append(services, map[string]interface{}{
					"type":    "dns",
					"address": service,
				})
			}
		}
		mask := translateMask(route.Prefix, true)
		routes = append(routes, map[string]interface{}{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  gateway,
			"services": services,
		})
	}
	return routes, nil
}

// getRoutesv6 returns the IPv6 routes.
func getRoutesv6(netRoutes []capm3.NetworkDataRoutev6,
	poolAddresses map[string]addressFromPool,
) ([]interface{}, error) {
	routes := []interface{}{}
	for _, route := range netRoutes {
		gateway := ipamv1.IPAddressv6Str("")
		if route.Gateway.String != nil {
			gateway = *route.Gateway.String
		} else if route.Gateway.FromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Gateway.FromIPPool]
			if !ok {
				return []interface{}{}, errors.New("Failed to fetch pool from cache")
			}
			gateway = ipamv1.IPAddressv6Str(poolAddress.gateway)
		}
		services := []interface{}{}
		for _, service := range route.Services.DNS {
			services = append(services, map[string]interface{}{
				"type":    "dns",
				"address": service,
			})
		}
		if route.Services.DNSFromIPPool != nil {
			poolAddress, ok := poolAddresses[*route.Services.DNSFromIPPool]
			if !ok {
				return []interface{}{}, errors.New("Pool not found in cache")
			}
			for _, service := range poolAddress.dnsServers {
				services = append(services, map[string]interface{}{
					"type":    "dns",
					"address": service,
				})
			}
		}
		mask := translateMask(route.Prefix, false)
		routes = append(routes, map[string]interface{}{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  gateway,
			"services": services,
		})
	}
	return routes, nil
}

// translateMask transforms a mask given as integer into a dotted-notation string.
func translateMask(maskInt int, ipv4 bool) interface{} {
	if ipv4 {
		// Get the mask by concatenating the IPv4 prefix of net package and the mask
		address := net.IP(append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255},
			[]byte(net.CIDRMask(maskInt, 32))...,
		)).String()
		return ipamv1.IPAddressv4Str(address)
	}
	// get the mask
	address := net.IP(net.CIDRMask(maskInt, 128)).String()
	return ipamv1.IPAddressv6Str(address)
}

// getLinkMacAddress returns the mac address.
func getLinkMacAddress(mac *capm3.NetworkLinkEthernetMac, bmh *bmo.BareMetalHost) (
	string, error,
) {
	macAddress := ""
	var err error

	// if a string was given
	if mac.String != nil {
		macAddress = *mac.String

		// Otherwise fetch the mac from the interface name
	} else if mac.FromHostInterface != nil {
		macAddress, err = getBMHMacByName(*mac.FromHostInterface, bmh)
	}

	return macAddress, err
}

// renderMetaData renders the MetaData items.
func renderMetaData(m3d *capm3.Metal3Data, m3dt *capm3.Metal3DataTemplate,
	m3m *capm3.Metal3Machine, machine *clusterv1.Machine, bmh *bmo.BareMetalHost,
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
		metadata[entry.Key] = string(poolAddress.address)
	}

	// Prefixes
	for _, entry := range m3dt.Spec.MetaData.PrefixesFromPool {
		poolAddress, ok := poolAddresses[entry.Name]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		metadata[entry.Key] = strconv.Itoa(poolAddress.prefix)
	}

	// Gateways
	for _, entry := range m3dt.Spec.MetaData.GatewaysFromPool {
		poolAddress, ok := poolAddresses[entry.Name]
		if !ok {
			return nil, errors.New("Pool not found in cache")
		}
		metadata[entry.Key] = string(poolAddress.gateway)
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
		switch strings.ToLower(entry.Object) {
		case m3machine:
			metadata[entry.Key] = m3m.Annotations[entry.Annotation]
		case capimachine:
			metadata[entry.Key] = machine.Annotations[entry.Annotation]
		case host:
			metadata[entry.Key] = bmh.Annotations[entry.Annotation]
		default:
			return nil, errors.New("Unknown object type")
		}
	}

	// Strings
	for _, entry := range m3dt.Spec.MetaData.Strings {
		metadata[entry.Key] = entry.Value
	}
	providerid := fmt.Sprintf("%s/%s/%s", m3dt.GetNamespace(), bmh.GetUID(), m3m.GetUID())
	metadata["providerid"] = providerid
	return yaml.Marshal(metadata)
}

// getBMHMacByName returns the mac address of the interface matching the name.
func getBMHMacByName(name string, bmh *bmo.BareMetalHost) (string, error) {
	if bmh == nil || bmh.Status.HardwareDetails == nil || bmh.Status.HardwareDetails.NIC == nil {
		return "", errors.New("Nics list not populated")
	}
	for _, nics := range bmh.Status.HardwareDetails.NIC {
		if nics.Name == name {
			return nics.MAC, nil
		}
	}
	return "", fmt.Errorf("nic name not found %v", name)
}

func (m *DataManager) getM3Machine(ctx context.Context, m3dt *capm3.Metal3DataTemplate) (*capm3.Metal3Machine, error) {
	if m.Data.Spec.Claim.Name == "" {
		return nil, errors.New("Claim name not set")
	}

	capm3DataClaim := &capm3.Metal3DataClaim{}
	claimNamespacedName := types.NamespacedName{
		Name:      m.Data.Spec.Claim.Name,
		Namespace: m.Data.Namespace,
	}

	if err := m.client.Get(ctx, claimNamespacedName, capm3DataClaim); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
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
		if ownerRef.Kind == "Metal3Machine" &&
			oGV.Group == capm3.GroupVersion.Group {
			metal3MachineName = ownerRef.Name
			break
		}
	}
	if metal3MachineName == "" {
		return nil, errors.New("Metal3Machine not found in owner references")
	}

	return getM3Machine(ctx, m.client, m.Log,
		metal3MachineName, m.Data.Namespace, m3dt, true,
	)
}

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
			mLog.Info("Address claim not found, requeuing")
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		err := errors.Wrap(err, "Failed to get address claim")
		return nil, err
	}
	return metal3IPClaim, nil
}
