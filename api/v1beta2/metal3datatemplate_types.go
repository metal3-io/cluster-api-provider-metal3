/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// dataTemplateFinalizer allows Metal3DataTemplateReconciler to clean up resources
	// associated with Metal3DataTemplate before removing it from the apiserver.
	DataTemplateFinalizer = "metal3datatemplate.infrastructure.cluster.x-k8s.io"
)

// MetaDataIndex contains the information to render the index.
type MetaDataIndex struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// offset is the offset to apply to the index when rendering it
	// +optional
	Offset int32 `json:"offset,omitempty"`
	// step is the multiplier of the index
	// +kubebuilder:default=1
	// +optional
	Step int32 `json:"step,omitempty"`
	// prefix is the prefix string
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// suffix is the suffix string
	// +optional
	Suffix string `json:"suffix,omitempty"`
}

// MetaDataFromLabel contains the information to fetch a label content, if the
// label does not exist, it is rendered as empty string.
type MetaDataFromLabel struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// object is the type of the object from which we retrieve the name
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	Object string `json:"object"`
	// label is the key of the label to fetch
	Label string `json:"label"`
}

// MetaDataFromAnnotation contains the information to fetch an annotation
// content, if the label does not exist, it is rendered as empty string.
type MetaDataFromAnnotation struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// object is the type of the object from which we retrieve the name
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	Object string `json:"object"`
	// annotation is the key of the annotation to fetch
	Annotation string `json:"annotation"`
}

// MetaDataString contains the information to render the string.
type MetaDataString struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// value is the string to render.
	Value string `json:"value"`
}

// MetaDataNamespace contains the information to render the namespace.
type MetaDataNamespace struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
}

// MetaDataObjectName contains the information to render the object name.
type MetaDataObjectName struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// object is the type of the object from which we retrieve the name
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	Object string `json:"object"`
}

// MetaDataHostInterface contains the information to render the object name.
type MetaDataHostInterface struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// interface is the name of the interface in the BareMetalHost Status Hardware
	// Details list of interfaces from which to fetch the MAC address.
	Interface string `json:"interface"`
}

// MetaDataIPAddress contains the info to render th ip address. It is IP-version
// agnostic.
type MetaDataIPAddress struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// start is the first ip address that can be rendered
	// +optional
	Start *ipamv1.IPAddressStr `json:"start,omitempty"`
	// end is the last IP address that can be rendered. It is used as a validation
	// that the rendered IP is in bound.
	// +optional
	End *ipamv1.IPAddressStr `json:"end,omitempty"`
	// subnet is used to validate that the rendered IP is in bounds. In case the
	// start value is not given, it is derived from the subnet ip incremented by 1
	// (`192.168.0.1` for `192.168.0.0/24`)
	// +optional
	Subnet *ipamv1.IPSubnetStr `json:"subnet,omitempty"`
	// step is the step between the IP addresses rendered.
	// +kubebuilder:default=1
	// +optional
	Step int32 `json:"step,omitempty"`
}

type FromPool struct {
	// key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`

	// name is the name of the IP pool used to fetch the value to set in the metadata map for cloud-init
	Name string `json:"name"`

	// apiGroup is the api group of the IP pool.
	APIGroup string `json:"apiGroup"`

	// kind is the kind of the IP pool
	Kind string `json:"kind"`
}

// FromPoolAnnotation contains the information to fetch pool reference details from an annotation.
type FromPoolAnnotation struct {
	// object is the type of the object from which we retrieve the annotation
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	Object string `json:"object"`

	// annotation is the key of the annotation that contains the pool name.
	// The annotation value should be a string containing the pool name.
	Annotation string `json:"annotation"`
}

// MetaData represents a keyand value of the metadata.
type MetaData struct {
	// strings is the list of metadata items to be rendered from strings
	// +optional
	Strings []MetaDataString `json:"strings,omitempty"`

	// objectNames is the list of metadata items to be rendered from the name
	// of objects.
	// +optional
	ObjectNames []MetaDataObjectName `json:"objectNames,omitempty"`

	// indexes is the list of metadata items to be rendered from the index of the
	// Metal3Data
	// +optional
	Indexes []MetaDataIndex `json:"indexes,omitempty"`

	// namespaces is the list of metadata items to be rendered from the namespace
	// +optional
	Namespaces []MetaDataNamespace `json:"namespaces,omitempty"`

	// ipAddressesFromPool is the list of metadata items to be rendered as ip addresses.
	// +optional
	IPAddressesFromPool []FromPool `json:"ipAddressesFromPool,omitempty"`

	// prefixesFromPool is the list of metadata items to be rendered as network prefixes.
	// +optional
	PrefixesFromPool []FromPool `json:"prefixesFromPool,omitempty"`

	// gatewaysFromPool is the list of metadata items to be rendered as gateway addresses.
	// +optional
	GatewaysFromPool []FromPool `json:"gatewaysFromPool,omitempty"`

	// dnsServersFromPool is the list of metadata items to be rendered as dns servers.
	// +optional
	DNSServersFromPool []FromPool `json:"dnsServersFromPool,omitempty"`

	// fromHostInterfaces is the list of metadata items to be rendered as MAC
	// addresses of the host interfaces.
	// +optional
	FromHostInterfaces []MetaDataHostInterface `json:"fromHostInterfaces,omitempty"`

	// fromLabels is the list of metadata items to be fetched from object labels
	// +optional
	FromLabels []MetaDataFromLabel `json:"fromLabels,omitempty"`

	// fromAnnotations is the list of metadata items to be fetched from object
	// Annotations
	// +optional
	FromAnnotations []MetaDataFromAnnotation `json:"fromAnnotations,omitempty"`
}

// NetworkLinkEthernetMacFromAnnotation contains the information to fetch an annotation
// content, if the label does not exist, it is rendered as empty string.
type NetworkLinkEthernetMacFromAnnotation struct {
	// object is the type of the object from which we retrieve the name
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	Object string `json:"object"`
	// annotation is the key of the annotation to fetch
	Annotation string `json:"annotation"`
}

// NetworkLinkEthernetMac represents the Mac address content.
type NetworkLinkEthernetMac struct {
	// string contains the MAC address given as a string
	// +optional
	String *string `json:"string,omitempty"`

	// fromHostInterface contains the name of the interface in the BareMetalHost
	// Introspection details from which to fetch the MAC address
	// +optional
	FromHostInterface *string `json:"fromHostInterface,omitempty"`

	// fromAnnotation references an object annotation to retrieve the
	// MAC address from
	// +optional
	FromAnnotation *NetworkLinkEthernetMacFromAnnotation `json:"fromAnnotation,omitempty"`
}

// NetworkDataLinkEthernet represents an ethernet link object.
type NetworkDataLinkEthernet struct {
	// type is the type of the ethernet link. It can be one of:
	// bridge, dvs, hw_veb, hyperv, ovs, tap, vhostuser, vif, phy
	// +kubebuilder:validation:Enum=bridge;dvs;hw_veb;hyperv;ovs;tap;vhostuser;vif;phy
	Type string `json:"type"`

	// id is the ID of the interface (used for naming)
	Id string `json:"id"` //nolint:stylecheck,revive

	// name is the interface name to be used by cloud-init. When combined with
	// MACAddress, cloud-init will rename the interface matching the MAC to this name.
	// When MACAddress is omitted, cloud-init will use this name directly.
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=`^[a-zA-Z][a-zA-Z0-9._-]*$`
	// +optional
	Name string `json:"name,omitempty"`

	// mtu is the MTU of the interface
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// +optional
	MTU int32 `json:"mtu,omitempty"`

	// macAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`
}

// NetworkDataLinkBond represents a bond link object.
type NetworkDataLinkBond struct {
	// bondMode is the mode of bond used. It can be one of
	// balance-rr, active-backup, balance-xor, broadcast, balance-tlb, balance-alb, 802.3ad
	// +kubebuilder:validation:Enum="balance-rr";"active-backup";"balance-xor";"broadcast";"balance-tlb";"balance-alb";"802.3ad"
	BondMode string `json:"bondMode"`

	// bondXmitHashPolicy selects the transmit hash policy used for port selection in balance-xor and 802.3ad modes
	// +kubebuilder:validation:Enum="layer2";"layer3+4";"layer2+3"
	// +optional
	BondXmitHashPolicy string `json:"bondXmitHashPolicy"`

	// id is the ID of the interface (used for naming)
	Id string `json:"id"` //nolint:stylecheck,revive

	// name is the interface name to be used by cloud-init. When combined with
	// macAddress, cloud-init will rename the interface matching the MAC to this name.
	// When macAddress is omitted, cloud-init will use this name directly.
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=`^[a-zA-Z][a-zA-Z0-9._-]*$`
	// +optional
	Name string `json:"name,omitempty"`

	// mtu is the MTU of the interface
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// +optional
	MTU int32 `json:"mtu,omitempty"`

	// macAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`

	// parameters blob passed without any validation/modifications into cloud-init config
	Parameters NetworkDataLinkBondParams `json:"parameters,omitempty"`

	// bondLinks is the list of links that are part of the bond.
	// +optional
	BondLinks []string `json:"bondLinks"`
}

// NetworkDataLinkBondParams represent the set of bond params.
type NetworkDataLinkBondParams map[string]apiextensionsv1.JSON

// NetworkDataLinkVlan represents a vlan link object.
type NetworkDataLinkVlan struct {
	// vlanID is the Vlan ID
	// +kubebuilder:validation:Maximum=4096
	VlanID int32 `json:"vlanID"`

	// id is the ID of the interface (used for naming)
	Id string `json:"id"` //nolint:stylecheck,revive

	// name is the interface name to be used by cloud-init. When combined with
	// macAddress, cloud-init will rename the interface matching the MAC to this name.
	// When macAddress is omitted, cloud-init will use this name directly.
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=`^[a-zA-Z][a-zA-Z0-9._-]*$`
	// +optional
	Name string `json:"name,omitempty"`

	// mtu is the MTU of the interface
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// +optional
	MTU int32 `json:"mtu,omitempty"`

	// macAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`

	// vlanLink is the name of the link on which the vlan should be added
	VlanLink string `json:"vlanLink"`
}

// NetworkDataLink contains list of different link objects.
type NetworkDataLink struct {
	// ethernets contains a list of Ethernet links
	// +optional
	Ethernets []NetworkDataLinkEthernet `json:"ethernets,omitempty"`

	// bonds contains a list of Bond links
	// +optional
	Bonds []NetworkDataLinkBond `json:"bonds,omitempty"`

	// vlans contains a list of Vlan links
	// +optional
	Vlans []NetworkDataLinkVlan `json:"vlans,omitempty"`
}

// NetworkDataService represents a service object.
type NetworkDataService struct {
	// dns is a list of DNS services
	// +optional
	DNS []ipamv1.IPAddressStr `json:"dns,omitempty"`

	// dnsFromIPPool is the name of the IPPool from which to get the DNS servers
	// +optional
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkDataServicev4 represents a service object.
type NetworkDataServicev4 struct {
	// dns is a list of IPv4 DNS services
	// +optional
	DNS []ipamv1.IPAddressv4Str `json:"dns,omitempty"`

	// dnsFromIPPool is the name of the IPPool from which to get the DNS servers
	// +optional
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkDataServicev6 represents a service object.
type NetworkDataServicev6 struct {
	// dns is a list of IPv6 DNS services
	// +optional
	DNS []ipamv1.IPAddressv6Str `json:"dns,omitempty"`

	// dnsFromIPPool is the name of the IPPool from which to get the DNS servers
	// +optional
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkGatewayv4 represents a gateway, given as a string or as a reference to
// a Metal3IPPool.
type NetworkGatewayv4 struct {
	// string is the gateway given as a string
	// +optional
	String *ipamv1.IPAddressv4Str `json:"string,omitempty"`

	// fromIPPool is the name of the IPPool to fetch the gateway from
	// +optional
	FromIPPool *string `json:"fromIPPool,omitempty"`

	// fromPoolRef is a reference to a IP pool to fetch the gateway from
	FromPoolRef *corev1.TypedLocalObjectReference `json:"fromPoolRef,omitempty"`

	// fromPoolAnnotation allows specifying the pool name via an annotation on
	// a Machine, Metal3Machine, or BareMetalHost object.
	// When set, fromIPPool and fromPoolRef are ignored.
	// +optional
	FromPoolAnnotation *FromPoolAnnotation `json:"fromPoolAnnotation,omitempty"`
}

// NetworkGatewayv6 represents a gateway, given as a string or as a reference to
// a Metal3IPPool.
type NetworkGatewayv6 struct {
	// string is the gateway given as a string
	// +optional
	String *ipamv1.IPAddressv6Str `json:"string,omitempty"`

	// fromIPPool is the name of the IPPool to fetch the gateway from
	// +optional
	FromIPPool *string `json:"fromIPPool,omitempty"`

	// fromPoolRef is a reference to a IP pool to fetch the gateway from
	FromPoolRef *corev1.TypedLocalObjectReference `json:"fromPoolRef,omitempty"`

	// fromPoolAnnotation allows specifying the pool name via an annotation on
	// a Machine, Metal3Machine, or BareMetalHost object.
	// When set, fromIPPool and fromPoolRef are ignored.
	// +optional
	FromPoolAnnotation *FromPoolAnnotation `json:"fromPoolAnnotation,omitempty"`
}

// NetworkDataRoutev4 represents an ipv4 route object.
type NetworkDataRoutev4 struct {
	// network is the IPv4 network address
	Network ipamv1.IPAddressv4Str `json:"network"`

	// prefix is the mask of the network as integer (max 32)
	// +kubebuilder:validation:Maximum=32
	// +optional
	Prefix int32 `json:"prefix,omitempty"`

	// gateway is the IPv4 address of the gateway
	Gateway NetworkGatewayv4 `json:"gateway"`

	// services is a list of IPv4 services
	// +optional
	Services NetworkDataServicev4 `json:"services,omitempty"`
}

// NetworkDataRoutev6 represents an ipv6 route object.
type NetworkDataRoutev6 struct {
	// network is the IPv6 network address
	Network ipamv1.IPAddressv6Str `json:"network"`

	// prefix is the mask of the network as integer (max 128)
	// +kubebuilder:validation:Maximum=128
	// +optional
	Prefix int32 `json:"prefix,omitempty"`

	// gateway is the IPv6 address of the gateway
	Gateway NetworkGatewayv6 `json:"gateway"`

	// services is a list of IPv6 services
	// +optional
	Services NetworkDataServicev6 `json:"services,omitempty"`
}

// NetworkDataIPv4 represents an ipv4 static network object.
type NetworkDataIPv4 struct {
	// id is the network ID (name)
	ID string `json:"id"`

	// link is the link on which the network applies
	Link string `json:"link"`

	// ipAddressFromIPPool contains the name of the IP pool to use to get an ip address
	IPAddressFromIPPool string `json:"ipAddressFromIPPool,omitempty"`

	// fromPoolRef is a reference to a IP pool to allocate an address from.
	FromPoolRef *corev1.TypedLocalObjectReference `json:"fromPoolRef,omitempty"`

	// fromPoolAnnotation allows specifying the pool reference via an annotation on
	// a Machine, Metal3Machine, or BareMetalHost object.
	// When set, ipAddressFromIPPool and fromPoolRef are ignored.
	// +optional
	FromPoolAnnotation *FromPoolAnnotation `json:"fromPoolAnnotation,omitempty"`

	// routes contains a list of IPv4 routes
	// +optional
	Routes []NetworkDataRoutev4 `json:"routes,omitempty"`
}

// NetworkDataIPv6 represents an ipv6 static network object.
type NetworkDataIPv6 struct {
	// id is the network ID (name)
	ID string `json:"id"`

	// link is the link on which the network applies
	Link string `json:"link"`

	// ipAddressFromIPPool contains the name of the IPPool to use to get an ip address
	IPAddressFromIPPool string `json:"ipAddressFromIPPool"`

	// fromPoolRef is a reference to a IP pool to allocate an address from.
	FromPoolRef *corev1.TypedLocalObjectReference `json:"fromPoolRef,omitempty"`

	// fromPoolAnnotation allows specifying the pool reference via an annotation on
	// a Machine, Metal3Machine, or BareMetalHost object.
	// When set, ipAddressFromIPPool and fromPoolRef are ignored.
	// +optional
	FromPoolAnnotation *FromPoolAnnotation `json:"fromPoolAnnotation,omitempty"`

	// routes contains a list of IPv6 routes
	// +optional
	Routes []NetworkDataRoutev6 `json:"routes,omitempty"`
}

// NetworkDataIPv4DHCP represents an ipv4 DHCP network object.
type NetworkDataIPv4DHCP struct {
	// id is the network ID (name)
	ID string `json:"id"`

	// link is the link on which the network applies
	Link string `json:"link"`

	// routes contains a list of IPv4 routes
	// +optional
	Routes []NetworkDataRoutev4 `json:"routes,omitempty"`
}

// NetworkDataIPv6DHCP represents an ipv6 DHCP network object.
type NetworkDataIPv6DHCP struct {
	// id is the network ID (name)
	ID string `json:"id"`

	// link is the link on which the network applies
	Link string `json:"link"`

	// routes contains a list of IPv6 routes
	// +optional
	Routes []NetworkDataRoutev6 `json:"routes,omitempty"`
}

// NetworkDataNetwork represents a network object.
type NetworkDataNetwork struct {
	// ipv4 contains a list of IPv4 static allocations
	// +optional
	IPv4 []NetworkDataIPv4 `json:"ipv4,omitempty"`

	// ipv6 contains a list of IPv6 static allocations
	// +optional
	IPv6 []NetworkDataIPv6 `json:"ipv6,omitempty"`

	// ipv4DHCP contains a list of IPv4 DHCP allocations
	// +optional
	IPv4DHCP []NetworkDataIPv4DHCP `json:"ipv4DHCP,omitempty"`

	// ipv6DHCP contains a list of IPv6 DHCP allocations
	// +optional
	IPv6DHCP []NetworkDataIPv6DHCP `json:"ipv6DHCP,omitempty"`

	// ipv6SLAAC contains a list of IPv6 SLAAC allocations
	// +optional
	IPv6SLAAC []NetworkDataIPv6DHCP `json:"ipv6SLAAC,omitempty"`
}

// NetworkData represents a networkData object.
type NetworkData struct {
	// links is a structure containing lists of different types objects
	// +optional
	Links NetworkDataLink `json:"links,omitempty"`

	// networks  is a structure containing lists of different types objects
	// +optional
	Networks NetworkDataNetwork `json:"networks,omitempty"`

	// services  is a structure containing lists of different types objects
	// +optional
	Services NetworkDataService `json:"services,omitempty"`
}

// Metal3DataTemplateSpec defines the desired state of Metal3DataTemplate.
type Metal3DataTemplateSpec struct {
	// clusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// metaData contains the information needed to generate the metadata secret
	// +optional
	MetaData *MetaData `json:"metaData,omitempty"`

	// networkData contains the information needed to generate the networkdata
	// secret
	// +optional
	NetworkData *NetworkData `json:"networkData,omitempty"`
}

// Metal3DataTemplateStatus defines the observed state of Metal3DataTemplate.
type Metal3DataTemplateStatus struct {
	// lastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// indexes contains the map of Metal3Machine and index used
	// +optional
	Indexes map[string]int32 `json:"indexes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datatemplates,scope=Namespaced,categories=cluster-api,shortName=m3dt;m3datatemplate;m3datatemplates;metal3dt;metal3datatemplate
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3DataTemplate"

// Metal3DataTemplate is the Schema for the metal3datatemplates API.
type Metal3DataTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// spec defines the desired state of Metal3DataTemplate.
	// +optional
	Spec Metal3DataTemplateSpec `json:"spec,omitempty"`
	// status defines the observed state of Metal3DataTemplate.
	// +optional
	Status Metal3DataTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3DataTemplateList contains a list of Metal3DataTemplate.
type Metal3DataTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3DataTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &Metal3DataTemplate{}, &Metal3DataTemplateList{})
}
