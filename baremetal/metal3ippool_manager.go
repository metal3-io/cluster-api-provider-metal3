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
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPPoolManagerInterface is an interface for a IPPoolManager
type IPPoolManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	SetClusterOwnerRef(*capi.Cluster) error
	UpdateAddresses(context.Context) (int, error)
}

// IPPoolManager is responsible for performing machine reconciliation
type IPPoolManager struct {
	client client.Client
	IPPool *capm3.Metal3IPPool
	Log    logr.Logger
}

// NewIPPoolManager returns a new helper for managing a ipPool object
func NewIPPoolManager(client client.Client,
	ipPool *capm3.Metal3IPPool, ipPoolLog logr.Logger) (*IPPoolManager, error) {

	return &IPPoolManager{
		client: client,
		IPPool: ipPool,
		Log:    ipPoolLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *IPPoolManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.IPPool.Finalizers, capm3.IPPoolFinalizer) {
		m.IPPool.Finalizers = append(m.IPPool.Finalizers,
			capm3.IPPoolFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *IPPoolManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.IPPool.Finalizers = Filter(m.IPPool.Finalizers,
		capm3.IPPoolFinalizer,
	)
}

func (m *IPPoolManager) SetClusterOwnerRef(cluster *capi.Cluster) error {
	if cluster == nil {
		return errors.New("Missing cluster")
	}
	// Verify that the owner reference is there, if not add it and update object,
	// if error requeue.
	_, err := findOwnerRefFromList(m.IPPool.OwnerReferences,
		cluster.TypeMeta, cluster.ObjectMeta)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
			return err
		}
		m.IPPool.OwnerReferences, err = setOwnerRefInList(
			m.IPPool.OwnerReferences, false, cluster.TypeMeta,
			cluster.ObjectMeta,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// RecreateStatus recreates the status if empty
func (m *IPPoolManager) getIndexes(ctx context.Context) (map[capm3.IPAddress]string, error) {

	m.Log.Info("Fetching Metal3IPAddress objects")

	//start from empty maps
	m.IPPool.Status.Allocations = make(map[string]capm3.IPAddress)

	addresses := make(map[capm3.IPAddress]string)

	for _, address := range m.IPPool.Spec.PreAllocations {
		addresses[address] = ""
	}

	// get list of Metal3IPAddress objects
	addressObjects := capm3.Metal3IPAddressList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err := m.client.List(ctx, &addressObjects, opts)
	if err != nil {
		return addresses, err
	}

	// Iterate over the Metal3IPAddress objects to find all addresses and objects
	for _, addressObject := range addressObjects.Items {

		// If IPPool does not point to this object, discard
		if addressObject.Spec.Pool.Name == "" {
			continue
		}
		if addressObject.Spec.Pool.Name != m.IPPool.Name {
			continue
		}

		// Get the claim Name, if unset use empty string, to still record the
		// index being used, to avoid conflicts
		claimName := ""
		if addressObject.Spec.Claim.Name != "" {
			claimName = addressObject.Spec.Claim.Name
		}
		m.IPPool.Status.Allocations[claimName] = addressObject.Spec.Address
		addresses[addressObject.Spec.Address] = claimName
	}
	m.updateStatusTimestamp()
	return addresses, nil
}

func (m *IPPoolManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.IPPool.Status.LastUpdated = &now
}

// UpdateAddresses manages the claims and creates or deletes Metal3IPAddress accordingly.
// It returns the number of current allocations
func (m *IPPoolManager) UpdateAddresses(ctx context.Context) (int, error) {

	addresses, err := m.getIndexes(ctx)
	if err != nil {
		return 0, err
	}

	// get list of Metal3IPClaim objects
	addressClaimObjects := capm3.Metal3IPClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err = m.client.List(ctx, &addressClaimObjects, opts)
	if err != nil {
		return 0, err
	}

	// Iterate over the Metal3IPClaim objects to find all addresses and objects
	for _, addressClaim := range addressClaimObjects.Items {
		// If IPPool does not point to this object, discard
		if addressClaim.Spec.Pool.Name != m.IPPool.Name {
			continue
		}

		if addressClaim.Status.Address != nil && addressClaim.DeletionTimestamp.IsZero() {
			continue
		}
		addresses, err = m.updateAddress(ctx, &addressClaim, addresses)
		if err != nil {
			return 0, err
		}
	}
	m.updateStatusTimestamp()
	return len(addresses), nil
}

func (m *IPPoolManager) updateAddress(ctx context.Context,
	addressClaim *capm3.Metal3IPClaim, addresses map[capm3.IPAddress]string,
) (map[capm3.IPAddress]string, error) {
	helper, err := patch.NewHelper(addressClaim, m.client)
	if err != nil {
		return addresses, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch addressClaim exiting this function so we can persist any changes.
	defer func() {
		fmt.Printf("\nPatching %v", addressClaim.Name)
		err := helper.Patch(ctx, addressClaim)
		if err != nil {
			m.Log.Info("failed to Patch capm3DataClaim")
		}
	}()

	addressClaim.Status.ErrorMessage = nil

	if addressClaim.DeletionTimestamp.IsZero() {
		addresses, err = m.createAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	} else {
		addresses, err = m.deleteAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	}
	return addresses, nil
}

func (m *IPPoolManager) allocateAddress(addressClaim *capm3.Metal3IPClaim,
	addresses map[capm3.IPAddress]string,
) (capm3.IPAddress, int, *capm3.IPAddress, error) {
	var err error

	// Get pre-allocated addresses
	allocatedAddress, ipAllocated := m.IPPool.Spec.PreAllocations[addressClaim.Name]
	// If the IP is pre-allocated, the default prefix and gateway are used
	prefix := m.IPPool.Spec.Prefix
	gateway := m.IPPool.Spec.Gateway

	for _, pool := range m.IPPool.Spec.Pools {
		if ipAllocated {
			break
		}
		if pool.Prefix != 0 {
			prefix = pool.Prefix
		}
		if pool.Gateway != nil {
			gateway = pool.Gateway
		}
		index := 0
		for !ipAllocated {
			allocatedAddress, err = getIPAddress(pool, index)
			fmt.Println(allocatedAddress)
			if err != nil {
				break
			}
			index++
			if _, ok := addresses[allocatedAddress]; !ok && allocatedAddress != "" {
				ipAllocated = true
			}
		}
	}
	if !ipAllocated {
		addressClaim.Status.ErrorMessage = pointer.StringPtr("Exhausted IP Pools")
		return "", 0, nil, errors.New("Exhausted IP Pools")
	}
	return allocatedAddress, prefix, gateway, nil
}

func (m *IPPoolManager) createAddress(ctx context.Context,
	addressClaim *capm3.Metal3IPClaim, addresses map[capm3.IPAddress]string,
) (map[capm3.IPAddress]string, error) {
	if !Contains(addressClaim.Finalizers, capm3.IPClaimFinalizer) {
		addressClaim.Finalizers = append(addressClaim.Finalizers,
			capm3.IPClaimFinalizer,
		)
	}

	if allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]; ok {
		formatedAddress := strings.Replace(
			strings.Replace(string(allocatedAddress), ":", "-", -1), ".", "-", -1,
		)
		addressClaim.Status.Address = &corev1.ObjectReference{
			Name:      m.IPPool.Spec.NamePrefix + "-" + formatedAddress,
			Namespace: m.IPPool.Namespace,
		}
		return addresses, nil
	}

	// Get a new index for this machine
	m.Log.Info("Getting address", "Claim", addressClaim.Name)
	// Get a new IP for this owner
	allocatedAddress, prefix, gateway, err := m.allocateAddress(addressClaim, addresses)
	if err != nil {
		return addresses, err
	}
	formatedAddress := strings.Replace(
		strings.Replace(string(allocatedAddress), ":", "-", -1), ".", "-", -1,
	)

	// Set the index and Metal3IPAddress names
	addressName := m.IPPool.Spec.NamePrefix + "-" + formatedAddress

	m.Log.Info("Address allocated", "Claim", addressClaim.Name, "address", allocatedAddress)

	// Create the Metal3IPAddress object, with an Owner ref to the Metal3Machine
	// (curOwnerRef) and to the Metal3IPPool
	addressObject := &capm3.Metal3IPAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Metal3IPAddress",
			APIVersion: capm3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      addressName,
			Namespace: m.IPPool.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					Controller: pointer.BoolPtr(true),
					APIVersion: m.IPPool.APIVersion,
					Kind:       m.IPPool.Kind,
					Name:       m.IPPool.Name,
					UID:        m.IPPool.UID,
				},
				metav1.OwnerReference{
					APIVersion: addressClaim.APIVersion,
					Kind:       addressClaim.Kind,
					Name:       addressClaim.Name,
					UID:        addressClaim.UID,
				},
			},
			Labels: addressClaim.Labels,
		},
		Spec: capm3.Metal3IPAddressSpec{
			Address: allocatedAddress,
			Pool: corev1.ObjectReference{
				Name:      m.IPPool.Name,
				Namespace: m.IPPool.Namespace,
			},
			Claim: corev1.ObjectReference{
				Name:      addressClaim.Name,
				Namespace: m.IPPool.Namespace,
			},
			Prefix:  prefix,
			Gateway: gateway,
		},
	}

	// Create the Metal3IPAddress object. If we get a conflict (that will set
	// HasRequeueAfterError), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(m.client, ctx, addressObject); err != nil {
		if _, ok := err.(*RequeueAfterError); !ok {
			addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to create associated Metal3IPAddress object")
		}
		return addresses, err
	}

	m.IPPool.Status.Allocations[addressClaim.Name] = allocatedAddress
	addresses[allocatedAddress] = addressClaim.Name

	addressClaim.Status.Address = &corev1.ObjectReference{
		Name:      addressName,
		Namespace: m.IPPool.Namespace,
	}

	return addresses, nil
}

// DeleteDatas deletes old secrets
func (m *IPPoolManager) deleteAddress(ctx context.Context,
	addressClaim *capm3.Metal3IPClaim, addresses map[capm3.IPAddress]string,
) (map[capm3.IPAddress]string, error) {

	m.Log.Info("Deleting Claim", "Metal3IPClaim", addressClaim.Name)

	allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]
	if ok {
		// Try to get the Metal3IPAddress. if it succeeds, delete it
		tmpM3Data := &capm3.Metal3IPAddress{}
		formatedAddress := strings.Replace(
			strings.Replace(string(allocatedAddress), ":", "-", -1), ".", "-", -1,
		)
		key := client.ObjectKey{
			Name:      m.IPPool.Spec.NamePrefix + "-" + formatedAddress,
			Namespace: m.IPPool.Namespace,
		}
		err := m.client.Get(ctx, key, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to get associated Metal3IPAddress object")
			return addresses, err
		} else if err == nil {
			// Delete the secret with metadata
			err = m.client.Delete(ctx, tmpM3Data)
			if err != nil && !apierrors.IsNotFound(err) {
				addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to delete associated Metal3IPAddress object")
				return addresses, err
			}
		}

	}
	addressClaim.Status.Address = nil
	addressClaim.Finalizers = Filter(addressClaim.Finalizers,
		capm3.IPClaimFinalizer,
	)

	m.Log.Info("Deleted Claim", "Metal3IPClaim", addressClaim.Name)

	if ok {
		if _, ok := m.IPPool.Spec.PreAllocations[addressClaim.Name]; !ok {
			delete(addresses, allocatedAddress)
		}
		delete(m.IPPool.Status.Allocations, addressClaim.Name)
	}
	m.updateStatusTimestamp()
	return addresses, nil
}
