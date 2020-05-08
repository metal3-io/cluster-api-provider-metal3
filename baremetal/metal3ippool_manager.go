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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPPoolManagerInterface is an interface for a IPPoolManager
type IPPoolManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	RecreateStatusConditionally(context.Context) error
	DeleteAddresses(context.Context) error
	CreateAddresses(context.Context) error
	DeleteReady() (bool, error)
}

// IPPoolManager is responsible for performing machine reconciliation
type IPPoolManager struct {
	client client.Client
	IPPool *capm3.Metal3IPPool
	Log    logr.Logger
}

// NewIPPoolManager returns a new helper for managing a dataTemplate object
func NewIPPoolManager(client client.Client,
	ipPool *capm3.Metal3IPPool, dataTemplateLog logr.Logger) (*IPPoolManager, error) {

	return &IPPoolManager{
		client: client,
		IPPool: ipPool,
		Log:    dataTemplateLog,
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

// RecreateStatusConditionally recreates the status if empty
func (m *IPPoolManager) RecreateStatusConditionally(ctx context.Context) error {
	// If the status is empty (lastUpdated not set), then either the object is new
	// or has been moved. In both case, Recreating the status will set LastUpdated
	// so we won't recreate afterwards.
	if m.IPPool.Status.LastUpdated.IsZero() {
		return m.RecreateStatus(ctx)
	}
	return nil
}

// RecreateStatus recreates the status if empty
func (m *IPPoolManager) RecreateStatus(ctx context.Context) error {

	m.Log.Info("Recreating the Metal3IPPool status")

	//start from empty maps
	m.IPPool.Status.Addresses = make(map[string]string)
	m.IPPool.Status.Allocations = make(map[string]string)

	// Include pre-allocated leases
	for ownerName, address := range m.IPPool.Spec.Allocations {
		m.IPPool.Status.Addresses[string(address)] = ownerName
	}

	// get list of Metal3Data objects
	ipObjects := capm3.Metal3IPAddressList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err := m.client.List(ctx, &ipObjects, opts)
	if err != nil {
		return err
	}

	// Iterate over the Metal3Data objects to find all indexes and objects
	for _, ipObject := range ipObjects.Items {

		// If IPPool does not point to this object, discard
		if ipObject.Spec.IPPool == nil {
			continue
		}
		if ipObject.Spec.IPPool.Name != m.IPPool.Name {
			continue
		}

		// Get the machine Name, if unset use empty string, to still record the
		// index being used, to avoid conflicts
		ownerName := ""
		if ipObject.Spec.Owner != nil {
			ownerName = ipObject.Spec.Owner.Name
		}
		m.IPPool.Status.Addresses[string(ipObject.Spec.Address)] = ownerName
		m.IPPool.Status.Allocations[ownerName] = ipObject.Name
	}

	m.updateStatusTimestamp()
	return nil
}

func (m *IPPoolManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.IPPool.Status.LastUpdated = &now
}

// CreateAddresses creates the missing secrets
func (m *IPPoolManager) CreateAddresses(ctx context.Context) error {

	requeueNeeded := false

	// Get the cluster name
	clusterName, ok := m.IPPool.Labels[capi.ClusterLabelName]
	if !ok {
		return errors.New("No cluster name found on Metal3IPPool object")
	}

	// Iterate over all ownerReferences
	for _, curOwnerRef := range m.IPPool.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return err
		}

		// If the owner is not a Metal3Machine of infrastructure.cluster.x-k8s.io
		// then discard
		if curOwnerRef.Kind == "Cluster" ||
			curOwnerRefGV.Group != capm3.GroupVersion.Group {
			continue
		}

		// If the owner already has an entry, discard
		if entry, ok := m.IPPool.Status.Allocations[curOwnerRef.Name]; ok {
			if entry != "" {
				continue
			}
		}

		if m.IPPool.Status.Addresses == nil {
			m.IPPool.Status.Addresses = make(map[string]string)
		}
		if m.IPPool.Status.Allocations == nil {
			m.IPPool.Status.Allocations = make(map[string]string)
		}

		// Get a new IP for this owner
		allocatedAddress, prefix, gateway, err := m.allocateAddress(curOwnerRef)
		if err != nil {
			m.IPPool.Status.Allocations[curOwnerRef.Name] = ""
			return err
		}

		// Set the index and Metal3Data names
		formatedIP := strings.Replace(strings.Replace(allocatedAddress, ":", "-", -1), ".", "-", -1)
		addressName := m.IPPool.Spec.NamePrefix + "-" + formatedIP
		m.IPPool.Status.Addresses[allocatedAddress] = curOwnerRef.Name
		m.IPPool.Status.Allocations[curOwnerRef.Name] = addressName

		m.Log.Info("IP Address allocated", "Owner", curOwnerRef.Name, "address", allocatedAddress)

		// Create the Metal3IPAddress object, with an Owner ref to the Owner
		// (curOwnerRef) and to the Metal3IPPool
		ipObject := &capm3.Metal3IPAddress{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Metal3IPAddress",
				APIVersion: capm3.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      addressName,
				Namespace: m.IPPool.Namespace,
				Labels: map[string]string{
					capi.ClusterLabelName: clusterName,
				},
				OwnerReferences: []metav1.OwnerReference{
					curOwnerRef, metav1.OwnerReference{
						Controller: pointer.BoolPtr(true),
						APIVersion: m.IPPool.APIVersion,
						Kind:       m.IPPool.Kind,
						Name:       m.IPPool.Name,
						UID:        m.IPPool.UID,
					},
				},
			},
			Spec: capm3.Metal3IPAddressSpec{
				IPPool: &corev1.ObjectReference{
					Name:      m.IPPool.Name,
					Namespace: m.IPPool.Namespace,
				},
				Owner: &corev1.ObjectReference{
					Name:      curOwnerRef.Name,
					Namespace: m.IPPool.Namespace,
				},
				Address: capm3.IPAddress(allocatedAddress),
				Prefix:  prefix,
				Gateway: gateway,
			},
		}

		// Create the Metal3IPAddress object. If we get a conflict (that will set
		// HasRequeueAfterError), then recreate the status because we are missing
		// an ip, then requeue to retrigger the reconciliation with the new state
		if err := createObject(m.client, ctx, ipObject); err != nil {
			if _, ok := errors.Cause(err).(HasRequeueAfterError); ok {
				if err := m.RecreateStatus(ctx); err != nil {
					return err
				}
				requeueNeeded = true
				continue
			} else {
				return err
			}
		}
	}
	m.updateStatusTimestamp()
	if requeueNeeded {
		return &RequeueAfterError{}
	}
	return nil
}

func (m *IPPoolManager) allocateAddress(curOwnerRef metav1.OwnerReference) (string, int, *capm3.IPAddress, error) {
	var err error
	ipAllocated := false
	allocatedAddress := ""
	tmpAllocatedAddress, ipAllocated := m.IPPool.Spec.Allocations[curOwnerRef.Name]
	// If the IP is pre-allocated, the default prefix and gateway are used
	if ipAllocated {
		allocatedAddress = string(tmpAllocatedAddress)
	}
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
		err = nil
		for err == nil && !ipAllocated {
			allocatedAddress, err = getIPAddress(pool, index)
			if err != nil {
				break
			}
			index++
			if _, ok := m.IPPool.Status.Addresses[allocatedAddress]; !ok && allocatedAddress != "" {
				ipAllocated = true
			}
		}
	}
	if !ipAllocated {
		return "", 0, nil, errors.New("Exhausted IP Pools")
	}

	fmt.Println(allocatedAddress, prefix, gateway, err)
	return allocatedAddress, prefix, gateway, nil
}

// DeleteAddresses deletes old secrets
func (m *IPPoolManager) DeleteAddresses(ctx context.Context) error {

	// Iterate over the Metal3Data objects
	for ownerName, addressName := range m.IPPool.Status.Allocations {
		present := false
		// Iterate over the owner Refs
		for _, curOwnerRef := range m.IPPool.ObjectMeta.OwnerReferences {
			curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
			if err != nil {
				return err
			}

			// If the owner ref is not a Metal3Machine, discard
			if curOwnerRef.Kind == "Cluster" ||
				curOwnerRefGV.Group != capm3.GroupVersion.Group {
				continue
			}

			// If the names match, the Metal3Data should be preserved
			if ownerName == curOwnerRef.Name {
				present = true
				break
			}
		}

		// Do not delete Metal3Data in use.
		if present {
			continue
		}

		m.Log.Info("Deleting Metal3IPAddress", "Owner", ownerName)

		// Try to get the Metal3Data. if it succeeds, delete it
		tmpM3IP := &capm3.Metal3IPAddress{}
		key := client.ObjectKey{
			Name:      addressName,
			Namespace: m.IPPool.Namespace,
		}
		err := m.client.Get(ctx, key, tmpM3IP)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		} else if err == nil {
			// Delete the secret with metadata
			err = m.client.Delete(ctx, tmpM3IP)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		deletionAddress := ""
		for address, name := range m.IPPool.Status.Addresses {
			if name == ownerName {
				deletionAddress = address
			}
		}
		delete(m.IPPool.Status.Addresses, deletionAddress)
		delete(m.IPPool.Status.Allocations, ownerName)
		m.Log.Info("IPAddress deleted", "Owner", ownerName)
	}
	m.updateStatusTimestamp()
	return nil
}

// DeleteRead returns true if the object is unreferenced (does not have
// Metal3Machine owner references)
func (m *IPPoolManager) DeleteReady() (bool, error) {
	for _, curOwnerRef := range m.IPPool.ObjectMeta.OwnerReferences {
		curOwnerRefGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return false, err
		}

		// If we still have a Metal3Machine owning this, do not delete
		if curOwnerRef.Kind != "Cluster" &&
			curOwnerRefGV.Group == capm3.GroupVersion.Group {
			return false, nil
		}
	}
	m.Log.Info("Metal3IPPool ready for deletion")
	return true, nil
}
