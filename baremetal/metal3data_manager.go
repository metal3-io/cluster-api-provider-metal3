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
	//"bytes"
	"context"
	"fmt"
	"strings"
	//"regexp"
	"strconv"
	//"text/template"
	"math/big"
	"net"

	// comment for go-lint
	"github.com/go-logr/logr"

	bmo "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// DataManagerInterface is an interface for a DataManager
type DataManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	CreateSecrets(ctx context.Context) error
}

// DataManager is responsible for performing machine reconciliation
type DataManager struct {
	client client.Client
	Data   *capm3.Metal3Data
	Log    logr.Logger
}

// NewDataManager returns a new helper for managing a Metal3Data object
func NewDataManager(client client.Client,
	data *capm3.Metal3Data, dataLog logr.Logger) (*DataManager, error) {

	return &DataManager{
		client: client,
		Data:   data,
		Log:    dataLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *DataManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.Data.Finalizers, capm3.DataFinalizer) {
		m.Data.Finalizers = append(m.Data.Finalizers,
			capm3.DataFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *DataManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	m.Data.Finalizers = Filter(m.Data.Finalizers,
		capm3.DataFinalizer,
	)
}

// CreateSecrets returns true if the object is
func (m *DataManager) CreateSecrets(ctx context.Context) error {
	var metaDataErr, networkDataErr error

	m3dt, err := m.fetchM3DataTemplate(ctx)
	if err != nil {
		return err
	}
	if m3dt == nil {
		return errors.New("Metal3DataTemplate unset")
	}
	m.Log.Info("Fetched Metal3DataTemplate")

	m3m, err := m.getM3Machine(ctx)
	if err != nil {
		return err
	}
	if m3m == nil {
		return errors.New("Metal3Machine unset")
	}
	m.Log.Info("Fetched Metal3Machine")

	if m3dt.Spec.MetaData != nil {
		if m.Data.Spec.MetaData == nil || m.Data.Spec.MetaData.Name == "" {
			m.Data.Spec.MetaData = &corev1.SecretReference{
				Name:      m3m.Name + "-metadata",
				Namespace: m.Data.Namespace,
			}
		}
		m.Log.Info("Checking if secret exists", "secret", m.Data.Spec.MetaData.Name)
		metaDataErr = checkSecretExists(m.client, ctx, m.Data.Spec.MetaData.Name,
			m.Data.Namespace,
		)

		if metaDataErr != nil && !apierrors.IsNotFound(metaDataErr) {
			return metaDataErr
		}
		if apierrors.IsNotFound(metaDataErr) {
			m.Log.Info("MetaData secret creation needed", "secret", m.Data.Spec.MetaData.Name)
		}
	}

	if m3dt.Spec.NetworkData != nil {
		if m.Data.Spec.NetworkData == nil || m.Data.Spec.NetworkData.Name == "" {
			m.Data.Spec.NetworkData = &corev1.SecretReference{
				Name:      m3m.Name + "-networkdata",
				Namespace: m.Data.Namespace,
			}
		}
		m.Log.Info("Checking if secret exists", "secret", m.Data.Spec.NetworkData.Name)
		networkDataErr = checkSecretExists(m.client, ctx, m.Data.Spec.NetworkData.Name,
			m.Data.Namespace,
		)
		if networkDataErr != nil && !apierrors.IsNotFound(networkDataErr) {
			return networkDataErr
		}
		if apierrors.IsNotFound(networkDataErr) {
			m.Log.Info("NetworkData secret creation needed", "secret", m.Data.Spec.NetworkData.Name)
		}
	}

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
		m.Log.Info("Waiting for Machine Controller to set OwnerRef on Metal3Machine")
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}

	m.Log.Info("Fetched Machine")

	bmh, err := getHost(ctx, m3m, m.client, m.Log)
	if err != nil {
		return err
	}
	if bmh == nil {
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}

	m.Log.Info("Fetched BMH")

	ownerRefs := []metav1.OwnerReference{
		metav1.OwnerReference{
			Controller: pointer.BoolPtr(true),
			APIVersion: m.Data.APIVersion,
			Kind:       m.Data.Kind,
			Name:       m.Data.Name,
			UID:        m.Data.UID,
		},
	}

	if apierrors.IsNotFound(metaDataErr) {
		m.Log.Info("Creating Metadata secret")
		metadata, err := renderMetaData(m.Data, m3dt, m3m, capiMachine, bmh)
		if err != nil {
			return err
		}
		fmt.Println(metadata)
		if err := createSecret(m.client, ctx, m.Data.Spec.MetaData.Name,
			m.Data.Namespace, m3dt.Labels[capi.ClusterLabelName],
			ownerRefs, map[string][]byte{"metaData": metadata},
		); err != nil {
			return err
		}
	}

	if apierrors.IsNotFound(networkDataErr) {
		m.Log.Info("Creating Networkdata secret")
		networkData, err := renderNetworkData(m.Data, m3dt, bmh)
		if err != nil {
			return err
		}
		fmt.Println(networkData)
		if err := createSecret(m.client, ctx, m.Data.Spec.NetworkData.Name,
			m.Data.Namespace, m3dt.Labels[capi.ClusterLabelName],
			ownerRefs, map[string][]byte{"networkData": networkData},
		); err != nil {
			return err
		}
	}

	m.Log.Info("Metal3Data reconciled")
	m.Data.Status.Ready = true
	return nil
}

func renderNetworkData(m3d *capm3.Metal3Data, m3dt *capm3.Metal3DataTemplate,
	bmh *bmo.BareMetalHost,
) ([]byte, error) {
	if m3dt.Spec.NetworkData == nil {
		return nil, nil
	}

	networkData := map[string][]interface{}{
		"links":    []interface{}{},
		"networks": []interface{}{},
		"services": []interface{}{},
	}
	for _, link := range m3dt.Spec.NetworkData.Links {
		if link.Ethernet != nil {
			mac_address, err := getLinkMacAddress(link.Ethernet.MACAddress, bmh)
			if err != nil {
				return nil, err
			}
			networkData["links"] = append(networkData["links"], map[string]interface{}{
				"type":                 link.Ethernet.Type,
				"id":                   link.Ethernet.Id,
				"mtu":                  link.Ethernet.MTU,
				"ethernet_mac_address": mac_address,
			})
		}
		if link.Bond != nil {
			mac_address, err := getLinkMacAddress(link.Bond.MACAddress, bmh)
			if err != nil {
				return nil, err
			}
			networkData["links"] = append(networkData["links"], map[string]interface{}{
				"type":                 "bond",
				"id":                   link.Bond.Id,
				"mtu":                  link.Bond.MTU,
				"ethernet_mac_address": mac_address,
				"bond_mode":            link.Bond.BondMode,
				"bond_links":           link.Bond.BondLinks,
			})
		}
		if link.Vlan != nil {
			mac_address, err := getLinkMacAddress(link.Vlan.MACAddress, bmh)
			if err != nil {
				return nil, err
			}
			networkData["links"] = append(networkData["links"], map[string]interface{}{
				"type":             "vlan",
				"id":               link.Vlan.Id,
				"mtu":              link.Vlan.MTU,
				"vlan_mac_address": mac_address,
				"vlan_id":          link.Vlan.VlanID,
				"vlan_link":        link.Vlan.VlanLink,
			})
		}
	}
	for _, network := range m3dt.Spec.NetworkData.Networks {
		if network.IPv4 != nil {
			mask := translateMask(network.IPv4.Netmask, true)
			ip, err := getIPAddress(&capm3.MetaDataIPAddress{
				Start:  &network.IPv4.IPAddress.Start,
				End:    &network.IPv4.IPAddress.End,
				Subnet: &network.IPv4.IPAddress.Subnet,
				Step:   network.IPv4.IPAddress.Step,
			}, m3d.Spec.Index,
			)
			if err != nil {
				return nil, err
			}
			routes := getRoutesv4(network.IPv4.Routes)
			networkData["networks"] = append(networkData["networks"], map[string]interface{}{
				"type":       "ipv4",
				"id":         network.IPv4.ID,
				"link":       network.IPv4.Link,
				"netmask":    mask,
				"ip_address": ip,
				"routes":     routes,
			})
		}
		if network.IPv6 != nil {
			mask := translateMask(network.IPv6.Netmask, false)
			ip, err := getIPAddress(&capm3.MetaDataIPAddress{
				Start:  &network.IPv6.IPAddress.Start,
				End:    &network.IPv6.IPAddress.End,
				Subnet: &network.IPv6.IPAddress.Subnet,
				Step:   network.IPv6.IPAddress.Step,
			}, m3d.Spec.Index,
			)
			if err != nil {
				return nil, err
			}
			routes := getRoutesv6(network.IPv6.Routes)
			networkData["networks"] = append(networkData["networks"], map[string]interface{}{
				"type":       "ipv6",
				"id":         network.IPv6.ID,
				"link":       network.IPv6.Link,
				"netmask":    mask,
				"ip_address": ip,
				"routes":     routes,
			})
		}
		if network.IPv4DHCP != nil {
			routes := getRoutesv4(network.IPv4DHCP.Routes)
			networkData["networks"] = append(networkData["networks"], map[string]interface{}{
				"type":   "ipv4_dhcp",
				"id":     network.IPv4DHCP.ID,
				"link":   network.IPv4DHCP.Link,
				"routes": routes,
			})
		}
		if network.IPv6DHCP != nil {
			routes := getRoutesv6(network.IPv6DHCP.Routes)
			networkData["networks"] = append(networkData["networks"], map[string]interface{}{
				"type":   "ipv6_dhcp",
				"id":     network.IPv6DHCP.ID,
				"link":   network.IPv6DHCP.Link,
				"routes": routes,
			})
		}
		if network.IPv6SLAAC != nil {
			routes := getRoutesv6(network.IPv6SLAAC.Routes)
			networkData["networks"] = append(networkData["networks"], map[string]interface{}{
				"type":   "ipv6_slaac",
				"id":     network.IPv6SLAAC.ID,
				"link":   network.IPv6SLAAC.Link,
				"routes": routes,
			})
		}
	}
	for _, service := range m3dt.Spec.NetworkData.Services {
		if service.DNS != nil {
			networkData["services"] = append(networkData["services"], map[string]string{
				"type":    "dns",
				"address": *service.DNS,
			})
		}
	}

	return yaml.Marshal(networkData)
}

func getRoutesv4(netRoutes []capm3.NetworkDataRoutev4) []interface{} {
	routes := []interface{}{}
	for _, route := range netRoutes {
		services := []map[string]string{}
		for _, service := range route.Services {
			services = append(services, map[string]string{
				"type":    "dns",
				"address": *service.DNS,
			})
		}
		mask := translateMask(route.Netmask, true)
		routes = append(routes, map[string]interface{}{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  route.Gateway,
			"services": services,
		})
	}
	return routes
}

func getRoutesv6(netRoutes []capm3.NetworkDataRoutev6) []interface{} {
	routes := []interface{}{}
	for _, route := range netRoutes {
		services := []map[string]string{}
		for _, service := range route.Services {
			services = append(services, map[string]string{
				"type":    "dns",
				"address": *service.DNS,
			})
		}
		mask := translateMask(route.Netmask, true)
		routes = append(routes, map[string]interface{}{
			"network":  route.Network,
			"netmask":  mask,
			"gateway":  route.Gateway,
			"services": services,
		})
	}
	return routes
}

func translateMask(maskInt int, ipv4 bool) string {
	maskBytes := make([]byte, 16)
	maskIP := net.IP(maskBytes)
	if ipv4 {
		maskIP[10] = 255
		maskIP[11] = 255
		copy(maskIP[12:], net.CIDRMask(maskInt, 32))
	} else {
		copy(maskIP, net.CIDRMask(maskInt, 128))
	}
	return maskIP.String()
}

func getLinkMacAddress(mac *capm3.NetworkLinkEthernetMac, bmh *bmo.BareMetalHost) (
	string, error,
) {
	mac_address := ""
	var err error
	if mac.String != nil {
		mac_address = *mac.String
	} else if mac.FromHostInterface != nil {
		mac_address, err = getBMHMacByName(*mac.FromHostInterface, bmh)
	}
	return mac_address, err
}

func renderMetaData(m3d *capm3.Metal3Data, m3dt *capm3.Metal3DataTemplate,
	m3m *capm3.Metal3Machine, machine *capi.Machine, bmh *bmo.BareMetalHost,
) ([]byte, error) {
	if m3dt.Spec.MetaData == nil {
		return nil, nil
	}
	metadata := make(map[string]string)
	var err error
	for _, entry := range m3dt.Spec.MetaData {
		value := ""
		if entry.FromHostInterface != nil {
			value, err = getBMHMacByName(*entry.FromHostInterface, bmh)
		}
		if entry.IPAddress != nil {
			value, err = getIPAddress(entry.IPAddress, m3d.Spec.Index)
		}
		if entry.Index != nil {
			value = strconv.Itoa(entry.Index.Offset + m3d.Spec.Index*entry.Index.Step)
		}
		if entry.ObjectName != nil {
			switch strings.ToLower(*entry.ObjectName) {
			case "metal3machine":
				value = m3m.Name
			case "machine":
				value = machine.Name
			case "baremetalhost":
				value = bmh.Name
			default:
				return nil, errors.New("Unknown object type")
			}
		}
		if entry.String != nil {
			value = *entry.String
		}
		if err != nil {
			return nil, err
		}
		metadata[entry.Key] = value
	}
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(metadata)
}

func getIPAddress(entry *capm3.MetaDataIPAddress, index int) (string, error) {
	if entry.Start == nil && entry.Subnet == nil {
		return "", errors.New("Either Start or Subnet is required for ipAddress")
	}
	var ip net.IP
	var err error
	offset := index * entry.Step
	if entry.Start != nil {
		var endIP net.IP
		if entry.End != nil {
			endIP = net.ParseIP(*entry.End)
		}
		ip, err = addOffsetToIP(net.ParseIP(*entry.Start), endIP, offset)
		if err != nil {
			return "", err
		}
	} else {
		var ipNet *net.IPNet
		ip, ipNet, err = net.ParseCIDR(*entry.Subnet)
		if err != nil {
			return "", err
		}
		offset++
		ip, err = addOffsetToIP(ip, nil, offset)
		if err != nil {
			return "", err
		}
		if !ipNet.Contains(ip) {
			return "", errors.New("IP address out of bonds")
		}
	}
	return ip.String(), nil
}

// Note that if the resulting IP address is in the format ::ffff:xxxx:xxxx then
// ip.String will fail to select the correct type of ip
func addOffsetToIP(ip, endIP net.IP, offset int) (net.IP, error) {
	ip4 := true
	//ip := net.ParseIP(ipString)
	if ip.To4() != nil {
		ip4 = true
	}
	IPInt := big.NewInt(0)
	OffsetInt := big.NewInt(int64(offset))
	IPInt = IPInt.SetBytes(ip)
	IPInt = IPInt.Add(IPInt, OffsetInt)
	IPBytes := IPInt.Bytes()
	IPBytesLen := len(IPBytes)
	fmt.Println(IPBytes)
	if (ip4 && IPBytesLen > 16 && IPBytes[9] == 255 && IPBytes[10] == 255) ||
		IPBytesLen > 16 {
		return nil, errors.New(fmt.Sprintf("IP address overflow for : %s", ip.String()))
	}
	if endIP != nil {
		endIPInt := big.NewInt(0)
		endIPInt = endIPInt.SetBytes(endIP)
		if IPInt.Cmp(endIPInt) > 0 {
			return nil, errors.New(fmt.Sprintf("IP address out of bonds for : %s", ip.String()))
		}
	}
	copy(ip[16-IPBytesLen:], IPBytes)
	return ip, nil
}

func getBMHMacByName(name string, bmh *bmo.BareMetalHost) (string, error) {
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

func (m *DataManager) getM3Machine(ctx context.Context) (*capm3.Metal3Machine, error) {

	if m.Data.Spec.Metal3Machine == nil {
		return nil, nil
	}
	if m.Data.Spec.Metal3Machine.Name == "" {
		return nil, errors.New("Metal3Machine name not set")
	}

	tmpM3Machine := &capm3.Metal3Machine{}
	key := client.ObjectKey{
		Name:      m.Data.Spec.Metal3Machine.Name,
		Namespace: m.Data.Namespace,
	}
	err := m.client.Get(ctx, key, tmpM3Machine)
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
	if tmpM3Machine.Spec.DataTemplate.Name != m.Data.Spec.DataTemplate.Name {
		return nil, nil
	}
	return tmpM3Machine, nil
}

// fetchMetadata fetches the Metal3DataTemplate object
func (m *DataManager) fetchM3DataTemplate(ctx context.Context) (*capm3.Metal3DataTemplate, error) {

	if m.Data.Spec.DataTemplate == nil {
		return nil, nil
	}
	if m.Data.Spec.DataTemplate.Name == "" {
		return nil, errors.New("Data Template name not set")
	}

	namespace := m.Data.Namespace

	if m.Data.Spec.DataTemplate.Namespace != "" {
		namespace = m.Data.Spec.DataTemplate.Namespace
	}
	// Fetch the Metal3 metadata.
	metal3DataTemplate := &capm3.Metal3DataTemplate{}
	metal3DataTemplateName := types.NamespacedName{
		Namespace: namespace,
		Name:      m.Data.Spec.DataTemplate.Name,
	}
	if err := m.client.Get(ctx, metal3DataTemplateName, metal3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			m.Log.Info("DataTemplate not found, requeuing")
			return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			err := errors.Wrap(err, "Failed to get data template")
			return nil, err
		}
	}
	return metal3DataTemplate, nil
}
