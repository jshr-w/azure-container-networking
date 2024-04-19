package network

import (
	"os"
	"strings"

	"github.com/Azure/azure-container-networking/netio"
	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/netns"
	"github.com/Azure/azure-container-networking/network/networkutils"
	"github.com/Azure/azure-container-networking/platform"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var errorSecondaryEndpointClient = errors.New("SecondaryEndpointClient Error")

func newErrorSecondaryEndpointClient(err error) error {
	return errors.Wrapf(err, "%s", errorSecondaryEndpointClient)
}

type SecondaryEndpointClient struct {
	netlink        netlink.NetlinkInterface
	netioshim      netio.NetIOInterface
	plClient       platform.ExecClient
	netUtilsClient networkutils.NetworkUtils
	nsClient       NamespaceClientInterface
	ep             *endpoint
}

func NewSecondaryEndpointClient(
	nl netlink.NetlinkInterface,
	nioc netio.NetIOInterface,
	plc platform.ExecClient,
	nsc NamespaceClientInterface,
	endpoint *endpoint,
) *SecondaryEndpointClient {
	client := &SecondaryEndpointClient{
		netlink:        nl,
		netioshim:      nioc,
		plClient:       plc,
		netUtilsClient: networkutils.NewNetworkUtils(nl, plc),
		nsClient:       nsc,
		ep:             endpoint,
	}

	return client
}

func (client *SecondaryEndpointClient) AddEndpoints(epInfo *EndpointInfo) error {
	iface, err := client.netioshim.GetNetworkInterfaceByMac(epInfo.MacAddress)
	if err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	epInfo.IfName = iface.Name
	if _, exists := client.ep.SecondaryInterfaces[iface.Name]; exists {
		return newErrorSecondaryEndpointClient(errors.New(iface.Name + " already exists"))
	}

	ipconfigs := make([]*IPConfig, len(epInfo.IPAddresses))
	for i, ipconfig := range epInfo.IPAddresses {
		ipconfigs[i] = &IPConfig{Address: ipconfig}
	}

	client.ep.SecondaryInterfaces[iface.Name] = &InterfaceInfo{
		Name:              iface.Name,
		MacAddress:        epInfo.MacAddress,
		IPConfigs:         ipconfigs,
		NICType:           epInfo.NICType,
		SkipDefaultRoutes: epInfo.SkipDefaultRoutes,
	}

	return nil
}

func (client *SecondaryEndpointClient) AddEndpointRules(_ *EndpointInfo) error {
	return nil
}

func (client *SecondaryEndpointClient) DeleteEndpointRules(_ *endpoint) {
}

func (client *SecondaryEndpointClient) MoveEndpointsToContainerNS(epInfo *EndpointInfo, nsID uintptr) error {
	// Move the container interface to container's network namespace.
	logger.Info("[net] Setting link %v netns %v.", zap.String("IfName", epInfo.IfName), zap.String("NetNsPath", epInfo.NetNsPath))
	if err := client.netlink.SetLinkNetNs(epInfo.IfName, nsID); err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	return nil
}

func (client *SecondaryEndpointClient) SetupContainerInterfaces(epInfo *EndpointInfo) error {
	logger.Info("[net] Setting link state up.", zap.String("IfName", epInfo.IfName))
	if err := client.netlink.SetLinkState(epInfo.IfName, true); err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	return nil
}

func (client *SecondaryEndpointClient) ConfigureContainerInterfacesAndRoutes(epInfo *EndpointInfo) error {
	if err := client.netUtilsClient.AssignIPToInterface(epInfo.IfName, epInfo.IPAddresses); err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	ifInfo, exists := client.ep.SecondaryInterfaces[epInfo.IfName]
	if !exists {
		return newErrorSecondaryEndpointClient(errors.New(epInfo.IfName + " does not exist"))
	}

	if len(epInfo.Routes) < 1 {
		return newErrorSecondaryEndpointClient(errors.New("routes expected for " + epInfo.IfName))
	}

	// virtual gw route needs to be scope link
	for i := range epInfo.Routes {
		if epInfo.Routes[i].Gw == nil {
			epInfo.Routes[i].Scope = netlink.RT_SCOPE_LINK
		}
	}

	if err := addRoutes(client.netlink, client.netioshim, epInfo.IfName, epInfo.Routes); err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	ifInfo.Routes = append(ifInfo.Routes, epInfo.Routes...)

	return nil
}

func (client *SecondaryEndpointClient) DeleteEndpoints(ep *endpoint) error {
	// Get VM namespace
	vmns, err := netns.New().Get()
	if err != nil {
		return newErrorSecondaryEndpointClient(err)
	}

	// Open the network namespace.
	logger.Info("Opening netns", zap.Any("NetNsPath", ep.NetworkNameSpace))
	ns, err := client.nsClient.OpenNamespace(ep.NetworkNameSpace)
	if err != nil {
		if strings.Contains(err.Error(), errFileNotExist.Error()) {
			// clear SecondaryInterfaces map since network namespace doesn't exist anymore
			ep.SecondaryInterfaces = make(map[string]*InterfaceInfo)
			return nil
		}

		return newErrorSecondaryEndpointClient(err)
	}
	defer ns.Close()

	// Enter the container network namespace.
	logger.Info("Entering netns", zap.Any("NetNsPath", ep.NetworkNameSpace))
	if err := ns.Enter(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ep.SecondaryInterfaces = make(map[string]*InterfaceInfo)
			return nil
		}

		return newErrorSecondaryEndpointClient(err)
	}

	// Return to host network namespace.
	defer func() {
		logger.Info("Exiting netns", zap.Any("NetNsPath", ep.NetworkNameSpace))
		if err := ns.Exit(); err != nil {
			logger.Error("Failed to exit netns with", zap.Error(newErrorSecondaryEndpointClient(err)))
		}
	}()

	for iface := range ep.SecondaryInterfaces {
		if err := client.netlink.SetLinkNetNs(iface, uintptr(vmns)); err != nil {
			logger.Error("Failed to move interface", zap.String("IfName", iface), zap.Error(newErrorSecondaryEndpointClient(err)))
			continue
		}

		delete(ep.SecondaryInterfaces, iface)
	}

	return nil
}
