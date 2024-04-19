package network

import (
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/log"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/ipam"
	"github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/platform"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/100"
	"go.uber.org/zap"
)

var logger = log.CNILogger.With(zap.String("component", "cni-net"))

const (
	bytesSize4  = 4
	bytesSize16 = 16
)

type AzureIPAMInvoker struct {
	plugin delegatePlugin
	nwInfo *network.NetworkInfo
}

type delegatePlugin interface {
	DelegateAdd(pluginName string, nwCfg *cni.NetworkConfig) (*cniTypesCurr.Result, error)
	DelegateDel(pluginName string, nwCfg *cni.NetworkConfig) error
	Errorf(format string, args ...interface{}) *cniTypes.Error
}

// Create an IPAM instance every time a CNI action is called.
func NewAzureIpamInvoker(plugin *NetPlugin, nwInfo *network.NetworkInfo) *AzureIPAMInvoker {
	return &AzureIPAMInvoker{
		plugin: plugin,
		nwInfo: nwInfo,
	}
}

func (invoker *AzureIPAMInvoker) Add(addConfig IPAMAddConfig) (IPAMAddResult, error) {
	addResult := IPAMAddResult{}

	if addConfig.nwCfg == nil {
		return addResult, invoker.plugin.Errorf("nil nwCfg passed to CNI ADD, stack: %+v", string(debug.Stack()))
	}

	if len(invoker.nwInfo.Subnets) > 0 {
		addConfig.nwCfg.IPAM.Subnet = invoker.nwInfo.Subnets[0].Prefix.String()
	}

	// Call into IPAM plugin to allocate an address pool for the network.
	result, err := invoker.plugin.DelegateAdd(addConfig.nwCfg.IPAM.Type, addConfig.nwCfg)
	if err != nil && strings.Contains(err.Error(), ipam.ErrNoAvailableAddressPools.Error()) {
		invoker.deleteIpamState()
		logger.Info("Retry pool allocation after deleting IPAM state")
		result, err = invoker.plugin.DelegateAdd(addConfig.nwCfg.IPAM.Type, addConfig.nwCfg)
	}

	if err != nil {
		err = invoker.plugin.Errorf("Failed to allocate pool: %v", err)
		return addResult, err
	}
	if len(result.IPs) > 0 {
		addResult.hostSubnetPrefix = result.IPs[0].Address
	}

	defer func() {
		if err != nil {
			if len(addResult.defaultInterfaceInfo.IPConfigs) > 0 {
				if er := invoker.Delete(&addResult.defaultInterfaceInfo.IPConfigs[0].Address, addConfig.nwCfg, nil, addConfig.options); er != nil {
					err = invoker.plugin.Errorf("Failed to clean up IP's during Delete with error %v, after Add failed with error %w", er, err)
				}
			} else {
				err = fmt.Errorf("No IP's to delete on error: %v", err)
			}
		}
	}()

	if addConfig.nwCfg.IPV6Mode != "" {
		nwCfg6 := *addConfig.nwCfg
		nwCfg6.IPAM.Environment = common.OptEnvironmentIPv6NodeIpam
		nwCfg6.IPAM.Type = ipamV6

		if len(invoker.nwInfo.Subnets) > 1 {
			// ipv6 is the second subnet of the slice
			nwCfg6.IPAM.Subnet = invoker.nwInfo.Subnets[1].Prefix.String()
		}

		var ipv6Result *cniTypesCurr.Result
		ipv6Result, err = invoker.plugin.DelegateAdd(nwCfg6.IPAM.Type, &nwCfg6)
		if err != nil {
			err = invoker.plugin.Errorf("Failed to allocate v6 pool: %v", err)
		} else {
			result.IPs = append(result.IPs, ipv6Result.IPs...)
			result.Routes = append(result.Routes, ipv6Result.Routes...)
			addResult.ipv6Enabled = true
		}
	}

	ipconfigs := make([]*network.IPConfig, len(result.IPs))
	for i, ipconfig := range result.IPs {
		ipconfigs[i] = &network.IPConfig{Address: ipconfig.Address, Gateway: ipconfig.Gateway}
	}

	routes := make([]network.RouteInfo, len(result.Routes))
	for i, route := range result.Routes {
		routes[i] = network.RouteInfo{Dst: route.Dst, Gw: route.GW}
	}

	addResult.defaultInterfaceInfo = network.InterfaceInfo{IPConfigs: ipconfigs, Routes: routes, DNS: network.DNSInfo{Suffix: result.DNS.Domain, Servers: result.DNS.Nameservers}, NICType: cns.InfraNIC}

	return addResult, err
}

func (invoker *AzureIPAMInvoker) deleteIpamState() {
	cniStateExists, err := platform.CheckIfFileExists(platform.CNIStateFilePath)
	if err != nil {
		logger.Error("Error checking CNI state exist", zap.Error(err))
		return
	}

	if cniStateExists {
		return
	}

	ipamStateExists, err := platform.CheckIfFileExists(platform.CNIIpamStatePath)
	if err != nil {
		logger.Error("Error checking IPAM state exist", zap.Error(err))
		return
	}

	if ipamStateExists {
		logger.Info("Deleting IPAM state file")
		err = os.Remove(platform.CNIIpamStatePath)
		if err != nil {
			logger.Error("Error deleting state file", zap.Error(err))
			return
		}
	}
}

func (invoker *AzureIPAMInvoker) Delete(address *net.IPNet, nwCfg *cni.NetworkConfig, _ *cniSkel.CmdArgs, options map[string]interface{}) error { //nolint
	if nwCfg == nil {
		return invoker.plugin.Errorf("nil nwCfg passed to CNI ADD, stack: %+v", string(debug.Stack()))
	}

	if len(invoker.nwInfo.Subnets) > 0 {
		nwCfg.IPAM.Subnet = invoker.nwInfo.Subnets[0].Prefix.String()
	}

	if address == nil {
		if err := invoker.plugin.DelegateDel(nwCfg.IPAM.Type, nwCfg); err != nil {
			return invoker.plugin.Errorf("Attempted to release address with error:  %v", err)
		}
	} else if len(address.IP.To4()) == bytesSize4 { //nolint:gocritic
		nwCfg.IPAM.Address = address.IP.String()
		logger.Info("Releasing ipv4",
			zap.String("address", nwCfg.IPAM.Address),
			zap.String("pool", nwCfg.IPAM.Subnet))
		if err := invoker.plugin.DelegateDel(nwCfg.IPAM.Type, nwCfg); err != nil {
			logger.Error("Failed to release ipv4 address", zap.Error(err))
			return invoker.plugin.Errorf("Failed to release ipv4 address: %v", err)
		}
	} else if len(address.IP.To16()) == bytesSize16 {
		nwCfgIpv6 := *nwCfg
		nwCfgIpv6.IPAM.Environment = common.OptEnvironmentIPv6NodeIpam
		nwCfgIpv6.IPAM.Type = ipamV6
		nwCfgIpv6.IPAM.Address = address.IP.String()
		if len(invoker.nwInfo.Subnets) > 1 {
			for _, subnet := range invoker.nwInfo.Subnets {
				if subnet.Prefix.IP.To4() == nil {
					nwCfgIpv6.IPAM.Subnet = subnet.Prefix.String()
					break
				}
			}
		}

		logger.Info("Releasing ipv6",
			zap.String("address", nwCfgIpv6.IPAM.Address),
			zap.String("pool", nwCfgIpv6.IPAM.Subnet))
		if err := invoker.plugin.DelegateDel(nwCfgIpv6.IPAM.Type, &nwCfgIpv6); err != nil {
			logger.Error("Failed to release ipv6 address", zap.Error(err))
			return invoker.plugin.Errorf("Failed to release ipv6 address: %v", err)
		}
	} else {
		return invoker.plugin.Errorf("Address is incorrect, not valid IPv4 or IPv6, stack: %+v", string(debug.Stack()))
	}

	return nil
}
