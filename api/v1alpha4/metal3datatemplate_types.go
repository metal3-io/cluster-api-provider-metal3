/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha4

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DataTemplateFinalizer allows Metal3DataTemplateReconciler to clean up resources
	// associated with Metal3DataTemplate before removing it from the apiserver.
	DataTemplateFinalizer = "metal3datatemplate.infrastructure.cluster.x-k8s.io"
)

// MetaDataObjectName contains the object from which to fetch the name
type MetaDataObjectName struct {
	Object string `json:"object"`
}

// MetaDataIndex contains the information to render the index
type MetaDataIndex struct {
	Offset int `json:"offset,omitempty"`
	// +kubebuilder:default=1
	Step int `json:"step,omitempty"`
}

// MetaDataIPAddress contains the info to render th ip address. It is IP-version
// agnostic
type MetaDataIPAddress struct {
	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	Start string `json:"start,omitempty"`
	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	End string `json:"end,omitempty"`
	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))/([0-9]|[1-2][0-9]|3[0-2])$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))/([0-9]|[0-9][0-9]|1[0-1][0-9]|12[0-8])$))"
	Subnet string `json:"subnet,omitempty"`
	// +kubebuilder:default=1
	Step int `json:"step,omitempty"`
}

// HostNicToMac contains the nic name to get the MAC address
type HostNicToMac struct {
	Name string `json:"name"`
}

// MetaDataEntry represents a keyand value of the metadata
type MetaDataEntry struct {
	Key               string              `json:"key"`
	String            string              `json:"string,omitempty"`
	ObjectName        *MetaDataObjectName `json:"objectName,omitempty"`
	Index             *MetaDataIndex      `json:"index,omitempty"`
	IPAddress         *MetaDataIPAddress  `json:"ipAddress,omitempty"`
	FromHostInterface *HostNicToMac       `json:"fromHostInterface,omitempty"`
}

// NetworkLinkEthernetMac represents the Mac address content
type NetworkLinkEthernetMac struct {
	String            string        `json:"string,omitempty"`
	FromHostInterface *HostNicToMac `json:"fromHostInterface,omitempty"`
}

// NetworkDataLinkEthernet represents an ethernet link object
type NetworkDataLinkEthernet struct {
	// +kubebuilder:validation.Enum=bridge,dvs,hw_veb,hyperv,ovs,tap,vhostuser,vif,phy
	Type string `json:"type"`
	Id   string `json:"id"`
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	MTU        int                     `json:"mtu,omitempty"`
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`
}

// NetworkDataLinkBond represents a bond link object
type NetworkDataLinkBond struct {
	// +kubebuilder:validation.Enum="802.1ad","balance-rr","active-backup","balance-xor","broadcast","balance-tlb","balance-alb"
	BondMode *string `json:"bondMode"`
	Id       string  `json:"id"`
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	MTU        int                     `json:"mtu,omitempty"`
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`
	BondLinks  []string                `json:"bondLinks"`
}

// NetworkDataLinkVlan represents a vlan link object
type NetworkDataLinkVlan struct {
	// +kubebuilder:validation:Maximum=4096
	VlanID int    `json:"vlanID"`
	Id     string `json:"id"`
	// +kubebuilder:default=1500
	// +kubebuilder:validation:Maximum=9000
	MTU        int                     `json:"mtu,omitempty"`
	MACAddress *NetworkLinkEthernetMac `json:"macAddress"`
	VlanLink   string                  `json:"vlanLink"`
}

// NetworkDataLink represents a link object
type NetworkDataLink struct {
	Ethernet *NetworkDataLinkEthernet `json:"ethernet,omitempty"`
	Bond     *NetworkDataLinkBond     `json:"bond,omitempty"`
	Vlan     *NetworkDataLinkVlan     `json:"vlan,omitempty"`
}

// NetworkDataService represents a service object
type NetworkDataService struct {
	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	DNS string `json:"dns,omitempty"`
}

// NetworkDataServicev4 represents a service object
type NetworkDataServicev4 struct {
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$"
	DNS string `json:"dns,omitempty"`
}

// NetworkDataServicev6 represents a service object
type NetworkDataServicev6 struct {
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$"
	DNS string `json:"dns,omitempty"`
}

// NetworkDataRoutev4 represents an ipv4 route object
type NetworkDataRoutev4 struct {
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$"
	Network string `json:"network,omitempty"`
	// +kubebuilder:validation:Maximum=32
	Netmask int `json:"netmask"`
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$"
	Gateway  string                 `json:"gateway,omitempty"`
	Services []NetworkDataServicev4 `json:"services,omitempty"`
}

// NetworkDataRoutev6 represents an ipv6 route object
type NetworkDataRoutev6 struct {
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$"
	Network string `json:"network,omitempty"`
	// +kubebuilder:validation:Maximum=128
	Netmask int `json:"netmask"`
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$"
	Gateway  string                 `json:"gateway,omitempty"`
	Services []NetworkDataServicev6 `json:"services,omitempty"`
}

// NetworkDataIPAddressv4 contains the info to render the ipv4 address.
type NetworkDataIPAddressv4 struct {
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$"
	Start string `json:"start,omitempty"`
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$"
	End string `json:"end,omitempty"`
	// +kubebuilder:validation:Pattern="^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))/([0-9]|[1-2][0-9]|3[0-2])$"
	Subnet string `json:"subnet,omitempty"`
	// +kubebuilder:default=1
	Step int `json:"step,omitempty"`
}

// NetworkDataIPAddressv6 contains the info to render the ipv6 address.
type NetworkDataIPAddressv6 struct {
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$"
	Start string `json:"start,omitempty"`
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$"
	End string `json:"end,omitempty"`
	// +kubebuilder:validation:Pattern="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))/([0-9]|[0-9][0-9]|1[0-1][0-9]|12[0-8])$"
	Subnet string `json:"subnet,omitempty"`
	// +kubebuilder:default=1
	Step int `json:"step,omitempty"`
}

// NetworkDataIPv4 represents an ipv4 static network object
type NetworkDataIPv4 struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	// +kubebuilder:validation:Maximum=32
	Netmask   int                    `json:"netmask"`
	IPAddress NetworkDataIPAddressv4 `json:"ipAddress"`
	Routes    []NetworkDataRoutev4   `json:"routes,omitempty"`
}

// NetworkDataIPv6 represents an ipv6 static network object
type NetworkDataIPv6 struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	// +kubebuilder:validation:Maximum=128
	Netmask   int                    `json:"netmask"`
	IPAddress NetworkDataIPAddressv6 `json:"ipAddress"`
	Routes    []NetworkDataRoutev6   `json:"routes,omitempty"`
}

// NetworkDataIPv4DHCP represents an ipv4 DHCP network object
type NetworkDataIPv4DHCP struct {
	ID     string               `json:"id"`
	Link   string               `json:"link"`
	Routes []NetworkDataRoutev4 `json:"routes,omitempty"`
}

// NetworkDataIPv6DHCP represents an ipv6 DHCP network object
type NetworkDataIPv6DHCP struct {
	ID     string               `json:"id"`
	Link   string               `json:"link"`
	Routes []NetworkDataRoutev6 `json:"routes,omitempty"`
}

// NetworkDataNetwork represents a network object
type NetworkDataNetwork struct {
	IPv4      *NetworkDataIPv4     `json:"ipv4,omitempty"`
	IPv6      *NetworkDataIPv6     `json:"ipv6,omitempty"`
	IPv4DHCP  *NetworkDataIPv4DHCP `json:"ipv4DHCP,omitempty"`
	IPv6DHCP  *NetworkDataIPv6DHCP `json:"ipv6DHCP,omitempty"`
	IPv6SLAAC *NetworkDataIPv6DHCP `json:"ipv6SLAAC,omitempty"`
}

// NetworkData represents a networkData object
type NetworkData struct {
	Links    []NetworkDataLink    `json:"links,omitempty"`
	Networks []NetworkDataNetwork `json:"networks,omitempty"`
	Services []NetworkDataService `json:"services,omitempty"`
}

// Metal3DataTemplateSpec defines the desired state of Metal3DataTemplate.
type Metal3DataTemplateSpec struct {
	MetaData    []MetaDataEntry `json:"metaData,omitempty"`
	NetworkData []NetworkData   `json:"networkData,omitempty"`
}

// Metal3DataTemplateSptatus defines the observed state of Metal3DataTemplate.
type Metal3DataTemplateStatus struct {
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	//Indexes contains the map of Metal3Machine and index used
	Indexes map[string]int `json:"indexes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=metal3datatemplates,scope=Namespaced,categories=cluster-api,shortName=m3dt;m3datatemplate
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"

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
