package e2e

import (
	"os"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"gopkg.in/yaml.v2"
)

type Network struct {
	// Name of the libvirt network.
	Name string `yaml:"name,omitempty"`
	// MacAddress of the interface connected to the network.
	MacAddress string `yaml:"macAddress,omitempty"`
	// IPAddress to reserve for the MAC address in the network.
	IPAddress string `yaml:"ipAddress,omitempty"`
}

// BMC defines a BMH to use in the tests.
type BMC struct {
	// User is the username for accessing the BMC.
	User string `yaml:"user,omitempty"`
	// Password is the password for accessing the BMC.
	Password string `yaml:"password,omitempty"`
	// Address of the BMC, e.g. "redfish-virtualmedia+http://192.168.222.1:8000/redfish/v1/Systems/bmo-e2e-1".
	Address string `yaml:"address,omitempty"`
	// DisableCertificateVerification indicates whether to disable certificate verification for the BMC connection.
	DisableCertificateVerification bool `yaml:"disableCertificateVerification,omitempty"`
	// BootMacAddress is the MAC address of the BMHs network interface.
	BootMacAddress string `yaml:"bootMacAddress,omitempty"`
	// BootMode is the boot mode for the BareMetalHost, e.g. "UEFI" or "legacy".
	BootMode bmov1alpha1.BootMode `yaml:"bootMode,omitempty"`
	// Name of the machine associated with this BMC.
	Name string `yaml:"name,omitempty"`
	// IPAddress is a reserved IP address for the BMH managed through this BMC.
	// This is used in tests that make ssh connections to the BMH.
	// Example: 192.168.222.122
	IPAddress string `yaml:"ipAddress,omitempty"`
	// RootDeviceHints provides guidance for where to write the disk image.
	RootDeviceHints bmov1alpha1.RootDeviceHints `yaml:"rootDeviceHints,omitempty"`
	// Networks describes the network interfaces that should be added to the VM representing this BMH.
	Networks []Network `yaml:"networks,omitempty"`
	// The Hostname of the node, which will be read into BMH object
	HostName string `yaml:"hostName,omitempty"`
	// The IP address of the node
	// Optional. Only needed if e2eConfig variable
	// SSH_CHECK_PROVISIONED is true
	SSHPort string `yaml:"sshPort,omitempty"`
}

func LoadBMCConfig(configPath string) (*[]BMC, error) {
	configData, err := os.ReadFile(configPath) //#nosec
	var bmcs []BMC
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(configData, &bmcs); err != nil {
		return nil, err
	}
	return &bmcs, nil
}
