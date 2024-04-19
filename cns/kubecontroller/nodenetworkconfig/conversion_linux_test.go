package nodenetworkconfig

import (
	"strconv"

	"github.com/Azure/azure-container-networking/cns"
)

var validOverlayRequest = &cns.CreateNetworkContainerRequest{
	HostPrimaryIP: validOverlayNC.NodeIP,
	Version:       strconv.FormatInt(0, 10),
	IPConfiguration: cns.IPConfiguration{
		IPSubnet: cns.IPSubnet{
			PrefixLength: uint8(subnetPrefixLen),
			IPAddress:    primaryIP,
		},
	},
	NetworkContainerid:   ncID,
	NetworkContainerType: cns.Docker,
	SecondaryIPConfigs: map[string]cns.SecondaryIPConfig{
		"10.0.0.0": {
			IPAddress: "10.0.0.0",
			NCVersion: 0,
		},
		"10.0.0.1": {
			IPAddress: "10.0.0.1",
			NCVersion: 0,
		},
		"10.0.0.2": {
			IPAddress: "10.0.0.2",
			NCVersion: 0,
		},
		"10.0.0.3": {
			IPAddress: "10.0.0.3",
			NCVersion: 0,
		},
	},
}

var validVNETBlockRequest = &cns.CreateNetworkContainerRequest{
	Version:       strconv.FormatInt(version, 10),
	HostPrimaryIP: vnetBlockNodeIP,
	IPConfiguration: cns.IPConfiguration{
		GatewayIPAddress: vnetBlockDefaultGateway,
		IPSubnet: cns.IPSubnet{
			PrefixLength: uint8(vnetBlockSubnetPrefixLen),
			IPAddress:    vnetBlockNodeIP,
		},
	},
	NetworkContainerid:   ncID,
	NetworkContainerType: cns.Docker,
	// Ignore first IP in first CIDR Block, i.e. 10.224.0.4
	SecondaryIPConfigs: map[string]cns.SecondaryIPConfig{
		"10.224.0.4": {
			IPAddress: "10.224.0.4",
			NCVersion: version,
		},
		"10.224.0.5": {
			IPAddress: "10.224.0.5",
			NCVersion: version,
		},
		"10.224.0.6": {
			IPAddress: "10.224.0.6",
			NCVersion: version,
		},
		"10.224.0.7": {
			IPAddress: "10.224.0.7",
			NCVersion: version,
		},
		"10.224.0.8": {
			IPAddress: "10.224.0.8",
			NCVersion: version,
		},
		"10.224.0.9": {
			IPAddress: "10.224.0.9",
			NCVersion: version,
		},
		"10.224.0.10": {
			IPAddress: "10.224.0.10",
			NCVersion: version,
		},
		"10.224.0.11": {
			IPAddress: "10.224.0.11",
			NCVersion: version,
		},
		"10.224.0.12": {
			IPAddress: "10.224.0.12",
			NCVersion: version,
		},
		"10.224.0.13": {
			IPAddress: "10.224.0.13",
			NCVersion: version,
		},
		"10.224.0.14": {
			IPAddress: "10.224.0.14",
			NCVersion: version,
		},
		"10.224.0.15": {
			IPAddress: "10.224.0.15",
			NCVersion: version,
		},
	},
}
