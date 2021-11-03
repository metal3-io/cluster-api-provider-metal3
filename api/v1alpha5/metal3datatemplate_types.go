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

package v1alpha5

import (
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DataTemplateFinalizer allows Metal3DataTemplateReconciler to clean up resources
	// associated with Metal3DataTemplate before removing it from the apiserver.
	DataTemplateFinalizer = "metal3datatemplate.infrastructure.cluster.x-k8s.io"
)

// MetaDataIndex contains the information to render the index
type MetaDataIndex struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// Offset is the offset to apply to the index when rendering it
	Offset int `json:"offset,omitempty"`
	// +kubebuilder:default=1
	// Step is the multiplier of the index
	Step int `json:"step,omitempty"`
	// Prefix is the prefix string
	Prefix string `json:"prefix,omitempty"`
	// Suffix is the suffix string
	Suffix string `json:"suffix,omitempty"`
}

// MetaDataFromLabel contains the information to fetch a label content, if the
// label does not exist, it is rendered as empty string
type MetaDataFromLabel struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	// Object is the type of the object from which we retrieve the name
	Object string `json:"object"`
	// Label is the key of the label to fetch
	Label string `json:"label"`
}

// MetaDataFromAnnotation contains the information to fetch an annotation
// content, if the label does not exist, it is rendered as empty string
type MetaDataFromAnnotation struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	// Object is the type of the object from which we retrieve the name
	Object string `json:"object"`
	// Annotation is the key of the Annotation to fetch
	Annotation string `json:"annotation"`
}

// MetaDataString contains the information to render the string
type MetaDataString struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// Value is the string to render.
	Value string `json:"value"`
}

// MetaDataNamespace contains the information to render the namespace
type MetaDataNamespace struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
}

// MetaDataObjectName contains the information to render the object name
type MetaDataObjectName struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// +kubebuilder:validation:Enum=machine;metal3machine;baremetalhost
	// Object is the type of the object from which we retrieve the name
	Object string `json:"object"`
}

// MetaDataHostInterface contains the information to render the object name
type MetaDataHostInterface struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// Interface is the name of the interface in the BareMetalHost Status Hardware
	// Details list of interfaces from which to fetch the MAC address.
	Interface string `json:"interface"`
}

// MetaDataIPAddress contains the info to render th ip address. It is IP-version
// agnostic
type MetaDataIPAddress struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`
	// Start is the first ip address that can be rendered
	Start *ipamv1.IPAddressStr `json:"start,omitempty"`
	// End is the last IP address that can be rendered. It is used as a validation
	// that the rendered IP is in bound.
	End *ipamv1.IPAddressStr `json:"end,omitempty"`
	// Subnet is used to validate that the rendered IP is in bounds. In case the
	// Start value is not given, it is derived from the subnet ip incremented by 1
	// (`192.168.0.1` for `192.168.0.0/24`)
	Subnet *ipamv1.IPSubnetStr `json:"subnet,omitempty"`
	// +kubebuilder:default=1
	// Step is the step between the IP addresses rendered.
	Step int `json:"step,omitempty"`
}

type FromPool struct {
	// Key will be used as the key to set in the metadata map for cloud-init
	Key string `json:"key"`

	// Name is the name of the IPPool used to fetch the value to set in the metadata map for cloud-init
	Name string `json:"name"`
}

// MetaData represents a keyand value of the metadata
type MetaData struct {
	// Strings is the list of metadata items to be rendered from strings
	Strings []MetaDataString `json:"strings,omitempty"`

	// ObjectNames is the list of metadata items to be rendered from the name
	// of objects.
	ObjectNames []MetaDataObjectName `json:"objectNames,omitempty"`

	// Indexes is the list of metadata items to be rendered from the index of the
	// Metal3Data
	Indexes []MetaDataIndex `json:"indexes,omitempty"`

	// Namespaces is the list of metadata items to be rendered from the namespace
	Namespaces []MetaDataNamespace `json:"namespaces,omitempty"`

	// IPAddressesFromPool is the list of metadata items to be rendered as ip addresses.
	IPAddressesFromPool []FromPool `json:"ipAddressesFromIPPool,omitempty"`

	// PrefixesFromPool is the list of metadata items to be rendered as network prefixes.
	PrefixesFromPool []FromPool `json:"prefixesFromIPPool,omitempty"`

	// GatewaysFromPool is the list of metadata items to be rendered as gateway addresses.
	GatewaysFromPool []FromPool `json:"gatewaysFromIPPool,omitempty"`

	// DNSServersFromPool is the list of metadata items to be rendered as dns servers.
	DNSServersFromPool []FromPool `json:"dnsServersFromIPPool,omitempty"`

	// FromHostInterfaces is the list of metadata items to be rendered as MAC
	// addresses of the host interfaces.
	FromHostInterfaces []MetaDataHostInterface `json:"fromHostInterfaces,omitempty"`

	// FromLabels is the list of metadata items to be fetched from object labels
	FromLabels []MetaDataFromLabel `json:"fromLabels,omitempty"`

	// FromAnnotations is the list of metadata items to be fetched from object
	// Annotations
	FromAnnotations []MetaDataFromAnnotation `json:"fromAnnotations,omitempty"`
}

// NetworkLinkEthernetMac represents the Mac address content
type NetworkLinkEthernetMac struct {
	// String contains the MAC address given as a string
	String *string `json:"string,omitempty"`

	// FromHostInterface contains the name of the interface in the BareMetalHost
	// Introspection details from which to fetch the MAC address
	FromHostInterface *string `json:"fromHostInterface,omitempty"`
}

// NetworkDataLinkEthernet represents an ethernet link object
type NetworkDataLinkEthernet struct {
	// +kubebuilder:validation:Enum=bridge;dvs;hw_veb;hyperv;ovs;tap;vhostuser;vif;phy
	// Type is the type of the ethernet link. It can be one of:
	// bridge, dvs, hw_veb, hyperv, ovs, tap, vhostuser, vif, phy
	Type string `json:"type"`

	// Id is the ID of the interface (used for naming)
	Id string `json:"id"`

	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// MTU is the MTU of the interface
	MTU int `json:"mtu,omitempty"`

	// MACAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`
}

// NetworkDataLinkBond represents a bond link object
type NetworkDataLinkBond struct {
	// +kubebuilder:validation:Enum="balance-rr";"active-backup";"balance-xor";"broadcast";"balance-tlb";"balance-alb";"802.3ad"
	// BondMode is the mode of bond used. It can be one of
	// balance-rr, active-backup, balance-xor, broadcast, balance-tlb, balance-alb, 802.3ad
	BondMode string `json:"bondMode"`

	// Id is the ID of the interface (used for naming)
	Id string `json:"id"`

	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// MTU is the MTU of the interface
	MTU int `json:"mtu,omitempty"`

	// MACAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`

	// BondLinks is the list of links that are part of the bond.
	BondLinks []string `json:"bondLinks"`
}

// NetworkDataLinkVlan represents a vlan link object
type NetworkDataLinkVlan struct {
	// +kubebuilder:validation:Maximum=4096
	// VlanID is the Vlan ID
	VlanID int `json:"vlanID"`

	// Id is the ID of the interface (used for naming)
	Id string `json:"id"`

	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	// MTU is the MTU of the interface
	MTU int `json:"mtu,omitempty"`

	// MACAddress is the MAC address of the interface, containing the object
	// used to render it.
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`

	// VlanLink is the name of the link on which the vlan should be added
	VlanLink string `json:"vlanLink"`
}

// NetworkDataLink contains list of different link objects
type NetworkDataLink struct {

	// Ethernets contains a list of Ethernet links
	Ethernets []NetworkDataLinkEthernet `json:"ethernets,omitempty"`

	//Bonds contains a list of Bond links
	Bonds []NetworkDataLinkBond `json:"bonds,omitempty"`

	// Vlans contains a list of Vlan links
	Vlans []NetworkDataLinkVlan `json:"vlans,omitempty"`
}

// NetworkDataService represents a service object
type NetworkDataService struct {

	// DNS is a list of DNS services
	DNS []ipamv1.IPAddressStr `json:"dns,omitempty"`

	//DNSFromIPPool is the name of the IPPool from which to get the DNS servers
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkDataServicev4 represents a service object
type NetworkDataServicev4 struct {
	// DNS is a list of IPv4 DNS services
	DNS []ipamv1.IPAddressv4Str `json:"dns,omitempty"`

	//DNSFromIPPool is the name of the IPPool from which to get the DNS servers
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkDataServicev6 represents a service object
type NetworkDataServicev6 struct {
	// DNS is a list of IPv6 DNS services
	DNS []ipamv1.IPAddressv6Str `json:"dns,omitempty"`

	//DNSFromIPPool is the name of the IPPool from which to get the DNS servers
	DNSFromIPPool *string `json:"dnsFromIPPool,omitempty"`
}

// NetworkGatewayv4 represents a gateway, given as a string or as a reference to
// a Metal3IPPool
type NetworkGatewayv4 struct {

	// String is the gateway given as a string
	String *ipamv1.IPAddressv4Str `json:"string,omitempty"`

	// FromIPPool is the name of the IPPool to fetch the gateway from
	FromIPPool *string `json:"fromIPPool,omitempty"`
}

// NetworkGatewayv6 represents a gateway, given as a string or as a reference to
// a Metal3IPPool
type NetworkGatewayv6 struct {

	// String is the gateway given as a string
	String *ipamv1.IPAddressv6Str `json:"string,omitempty"`

	// FromIPPool is the name of the IPPool to fetch the gateway from
	FromIPPool *string `json:"fromIPPool,omitempty"`
}

// NetworkDataRoutev4 represents an ipv4 route object
type NetworkDataRoutev4 struct {
	// Network is the IPv4 network address
	Network ipamv1.IPAddressv4Str `json:"network"`

	// +kubebuilder:validation:Maximum=32
	// Prefix is the mask of the network as integer (max 32)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the IPv4 address of the gateway
	Gateway NetworkGatewayv4 `json:"gateway"`

	//Services is a list of IPv4 services
	Services NetworkDataServicev4 `json:"services,omitempty"`
}

// NetworkDataRoutev6 represents an ipv6 route object
type NetworkDataRoutev6 struct {
	// Network is the IPv6 network address
	Network ipamv1.IPAddressv6Str `json:"network"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the IPv6 address of the gateway
	Gateway NetworkGatewayv6 `json:"gateway"`

	//Services is a list of IPv6 services
	Services NetworkDataServicev6 `json:"services,omitempty"`
}

// NetworkDataIPv4 represents an ipv4 static network object
type NetworkDataIPv4 struct {

	// ID is the network ID (name)
	ID string `json:"id"`

	// Link is the link on which the network applies
	Link string `json:"link"`

	// IPAddressFromIPPool contains the name of the IPPool to use to get an ip address
	IPAddressFromIPPool string `json:"ipAddressFromIPPool"`

	// Routes contains a list of IPv4 routes
	Routes []NetworkDataRoutev4 `json:"routes,omitempty"`
}

// NetworkDataIPv6 represents an ipv6 static network object
type NetworkDataIPv6 struct {

	// ID is the network ID (name)
	ID string `json:"id"`

	// Link is the link on which the network applies
	Link string `json:"link"`

	// IPAddressFromIPPool contains the name of the IPPool to use to get an ip address
	IPAddressFromIPPool string `json:"ipAddressFromIPPool"`

	// Routes contains a list of IPv6 routes
	Routes []NetworkDataRoutev6 `json:"routes,omitempty"`
}

// NetworkDataIPv4DHCP represents an ipv4 DHCP network object
type NetworkDataIPv4DHCP struct {

	// ID is the network ID (name)
	ID string `json:"id"`

	// Link is the link on which the network applies
	Link string `json:"link"`

	// Routes contains a list of IPv4 routes
	Routes []NetworkDataRoutev4 `json:"routes,omitempty"`
}

// NetworkDataIPv6DHCP represents an ipv6 DHCP network object
type NetworkDataIPv6DHCP struct {

	// ID is the network ID (name)
	ID string `json:"id"`

	// Link is the link on which the network applies
	Link string `json:"link"`

	// Routes contains a list of IPv6 routes
	Routes []NetworkDataRoutev6 `json:"routes,omitempty"`
}

// NetworkDataNetwork represents a network object
type NetworkDataNetwork struct {

	// IPv4 contains a list of IPv4 static allocations
	IPv4 []NetworkDataIPv4 `json:"ipv4,omitempty"`

	// IPv4 contains a list of IPv6 static allocations
	IPv6 []NetworkDataIPv6 `json:"ipv6,omitempty"`

	// IPv4 contains a list of IPv4 DHCP allocations
	IPv4DHCP []NetworkDataIPv4DHCP `json:"ipv4DHCP,omitempty"`

	// IPv4 contains a list of IPv6 DHCP allocations
	IPv6DHCP []NetworkDataIPv6DHCP `json:"ipv6DHCP,omitempty"`

	// IPv4 contains a list of IPv6 SLAAC allocations
	IPv6SLAAC []NetworkDataIPv6DHCP `json:"ipv6SLAAC,omitempty"`
}

// NetworkData represents a networkData object
type NetworkData struct {
	// Links is a structure containing lists of different types objects
	Links NetworkDataLink `json:"links,omitempty"`

	//Networks  is a structure containing lists of different types objects
	Networks NetworkDataNetwork `json:"networks,omitempty"`

	//Services  is a structure containing lists of different types objects
	Services NetworkDataService `json:"services,omitempty"`
}

// Metal3DataTemplateSpec defines the desired state of Metal3DataTemplate.
type Metal3DataTemplateSpec struct {

	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// TemplateReference refers to the Template the Metal3MachineTemplate refers to.
	// It can be matched against the key or it may also point to the name of the template
	// Metal3Data refers to
	TemplateReference string `json:"templateReference,omitempty"`

	//MetaData contains the information needed to generate the metadata secret
	MetaData *MetaData `json:"metaData,omitempty"`

	//NetworkData contains the information needed to generate the networkdata
	// secret
	NetworkData *NetworkData `json:"networkData,omitempty"`
}

// Metal3DataTemplateStatus defines the observed state of Metal3DataTemplate.
type Metal3DataTemplateStatus struct {
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	//Indexes contains the map of Metal3Machine and index used
	Indexes map[string]int `json:"indexes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datatemplates,scope=Namespaced,categories=cluster-api,shortName=m3dt;m3datatemplate;m3datatemplates;metal3dt;metal3datatemplate
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3DataTemplate"

// Metal3DataTemplate is the Schema for the metal3datatemplates API
type Metal3DataTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Metal3DataTemplateSpec   `json:"spec,omitempty"`
	Status Metal3DataTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3DataTemplateList contains a list of Metal3DataTemplate
type Metal3DataTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3DataTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3DataTemplate{}, &Metal3DataTemplateList{})
}
