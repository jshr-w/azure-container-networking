package network

import (
	"net"
	"testing"

	"github.com/Azure/azure-container-networking/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Suite")
}

var _ = Describe("Test Network", func() {
	Describe("Test newExternalInterface", func() {
		Context("When external interface already exists", func() {
			It("Should return nil without update", func() {
				ifName := "eth0"
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces[ifName] = &externalInterface{
					Name:    ifName,
					Subnets: []string{"10.0.0.0/16"},
				}
				err := nm.newExternalInterface(ifName, "10.1.0.0/16")
				Expect(err).To(BeNil())
				Expect(nm.ExternalInterfaces[ifName].Subnets).To(ContainElement("10.0.0.0/16"))
				Expect(nm.ExternalInterfaces[ifName].Subnets).NotTo(ContainElement("10.1.0.0/16"))
			})
		})
	})

	Describe("Test deleteExternalInterface", func() {
		Context("Delete external interface from network manager", func() {
			It("Interface should be deleted", func() {
				ifName := "eth0"
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces[ifName] = &externalInterface{
					Name:    ifName,
					Subnets: []string{"10.0.0.0/16", "10.1.0.0/16"},
				}
				err := nm.deleteExternalInterface(ifName)
				Expect(err).NotTo(HaveOccurred())
				Expect(nm.ExternalInterfaces[ifName]).To(BeNil())
			})
		})
	})

	Describe("Test findExternalInterfaceBySubnet", func() {
		Context("When subnet was found or nor found in external interfaces", func() {
			It("Should return the external interface when found and nil when not found", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Name:    "eth0",
					Subnets: []string{"subnet1", "subnet2"},
				}
				nm.ExternalInterfaces["en0"] = &externalInterface{
					Name:    "en0",
					Subnets: []string{"subnet3", "subnet4"},
				}
				exInterface := nm.findExternalInterfaceBySubnet("subnet4")
				Expect(exInterface.Name).To(Equal("en0"))
				exInterface = nm.findExternalInterfaceBySubnet("subnet0")
				Expect(exInterface).To(BeNil())
			})
		})
	})

	Describe("Test findExternalInterfaceByName", func() {
		Context("When ifName found or nor found", func() {
			It("Should return the external interface when found and nil when not found", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Name: "eth0",
				}
				nm.ExternalInterfaces["en0"] = nil
				exInterface := nm.findExternalInterfaceByName("eth0")
				Expect(exInterface.Name).To(Equal("eth0"))
				exInterface = nm.findExternalInterfaceByName("en0")
				Expect(exInterface).To(BeNil())
				exInterface = nm.findExternalInterfaceByName("lo")
				Expect(exInterface).To(BeNil())
			})
		})
	})

	Describe("Test newNetwork", func() {
		Context("When nwInfo.Mode is empty", func() {
			It("Should set as defalut mode", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nwInfo := &NetworkInfo{
					MasterIfName: "eth0",
				}
				_, _ = nm.newNetwork(nwInfo)
				Expect(nwInfo.Mode).To(Equal(opModeDefault))
			})
		})

		Context("When extIf not found by name", func() {
			It("Should raise errSubnetNotFound", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nwInfo := &NetworkInfo{
					MasterIfName: "eth0",
				}
				nw, err := nm.newNetwork(nwInfo)
				Expect(err).To(Equal(errSubnetNotFound))
				Expect(nw).To(BeNil())
			})
		})

		Context("When extIf not found by subnet", func() {
			It("Should raise errSubnetNotFound", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nwInfo := &NetworkInfo{
					Subnets: []SubnetInfo{{
						Prefix: net.IPNet{
							IP:   net.IPv4(10, 0, 0, 1),
							Mask: net.IPv4Mask(255, 255, 0, 0),
						},
					}},
				}
				nw, err := nm.newNetwork(nwInfo)
				Expect(err).To(Equal(errSubnetNotFound))
				Expect(nw).To(BeNil())
			})
		})

		Context("When network already exist", func() {
			It("Should raise errNetworkExists", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Networks: map[string]*network{},
				}
				nm.ExternalInterfaces["eth0"].Networks["nw"] = &network{}
				nwInfo := &NetworkInfo{
					Id:           "nw",
					MasterIfName: "eth0",
				}
				nw, err := nm.newNetwork(nwInfo)
				Expect(err).To(Equal(errNetworkExists))
				Expect(nw).To(BeNil())
			})
		})

		Context("create new network in transparent mode", func() {
			It("Should create new network", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
					plClient:           platform.NewMockExecClient(false),
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Networks: map[string]*network{},
				}
				nwInfo := &NetworkInfo{
					Id:           "nw",
					MasterIfName: "eth0",
					Mode:         opModeTransparent,
					IPV6Mode:     IPV6Nat,
				}
				nw, err := nm.newNetwork(nwInfo)
				Expect(err).To(BeNil())
				Expect(nw).NotTo(BeNil())
				Expect(nw.Id).To(Equal(nwInfo.Id))
			})
		})

		Context("When we cannot enable ipv4 forwarding", func() {
			It("Should error when ipv4 forwarding failed", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
					plClient:           platform.NewMockExecClient(true),
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Networks: map[string]*network{},
				}
				nwInfo := &NetworkInfo{
					Id:           "nw",
					MasterIfName: "eth0",
					Mode:         opModeTransparentVlan,
				}
				nw, err := nm.newNetwork(nwInfo)
				Expect(err).To(MatchError(platform.ErrMockExec))
				Expect(nw).To(BeNil())
			})
		})
	})

	Describe("Test deleteNetwork", func() {
		Context("When network not found", func() {
			It("Should raise errNetworkNotFound", func() {
				nm := &networkManager{}
				err := nm.deleteNetwork("invalid")
				Expect(err).To(Equal(errNetworkNotFound))
			})
		})
	})

	Describe("Test getNetwork", func() {
		Context("When network found or nor found", func() {
			It("Should return the network when found and nil when not found", func() {
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{},
				}
				nm.ExternalInterfaces["eth0"] = &externalInterface{
					Name:     "eth0",
					Networks: map[string]*network{},
				}
				nm.ExternalInterfaces["eth0"].Networks["nw1"] = &network{}
				nw, err := nm.getNetwork("nw1")
				Expect(err).NotTo(HaveOccurred())
				Expect(nw).NotTo(BeNil())
				nw, err = nm.getNetwork("invalid")
				Expect(err).To(Equal(errNetworkNotFound))
				Expect(nw).To(BeNil())
			})
		})
	})

	Describe("Test FindNetworkIDFromNetNs", func() {
		Context("When network exists", func() {
			It("Should be returned", func() {
				netNs := "989c079b-45a6-485f-8f9e-88b05d6c55c4"
				networkID := "byovnetbridge-vlan1-10-128-8-0_23"
				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{
						networkID: {
							Name: networkID,
							Networks: map[string]*network{
								"byovnetbridge-vlan1-10-128-8-0_23": {
									Id: "byovnetbridge-vlan1-10-128-8-0_23",
									Endpoints: map[string]*endpoint{
										"a591be2a-eth0": {
											Id:    "a591be2a-eth0",
											NetNs: netNs,
										},
									},
									NetNs: "aaac079b-45a6-485f-8f9e-88b05d6c55c4",
								},
							},
						},
					},
				}

				got, err := nm.FindNetworkIDFromNetNs(netNs)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(networkID))
			})
		})

		Context("When network does not exist", func() {
			It("Should return an errNetworkNotFound", func() {
				nm := &networkManager{
					ExternalInterfaces: make(map[string]*externalInterface),
				}

				_, err := nm.FindNetworkIDFromNetNs("989c079b-45a6-485f-8f9e-88b05d6c55c4")
				Expect(err).To(HaveOccurred())
				Expect(IsNetworkNotFoundError(err)).To(BeTrue())
			})
		})
	})

	Describe("Test GetNumEndpointsByContainerID", func() {
		Context("When one network has one endpoint and another network has two endpoints", func() {
			It("Should return three endpoints", func() {
				containerID := "989c079b-45a6-485f-8f9e-88b05d6c55c5"
				networkOneID := "byovnetbridge-vlan1-10-128-8-0_23"
				networkTwoID := "byovnetbridge-vlan2-20-128-8-0_23"

				nm := &networkManager{
					ExternalInterfaces: map[string]*externalInterface{
						networkOneID: {
							Name: networkOneID,
							Networks: map[string]*network{
								"byovnetbridge-vlan1-10-128-8-0_23": {
									Id: "byovnetbridge-vlan1-10-128-8-0_23",
									Endpoints: map[string]*endpoint{
										"a591be2a-eth0": {
											Id:          "a591be2a-eth0",
											ContainerID: containerID,
										},
										"a691be2b-eth0": {
											Id:          "a691be2b-eth0",
											ContainerID: containerID,
										},
									},
								},
							},
						},
						networkTwoID: {
							Name: networkTwoID,
							Networks: map[string]*network{
								"byovnetbridge-vlan2-20-128-8-0_23": {
									Id: "byovnetbridge-vlan2-20-128-8-0_23",
									Endpoints: map[string]*endpoint{
										"a591be2b-eth0": {
											Id:          "a591be2b-eth0",
											ContainerID: containerID,
										},
									},
									NetNs: "",
								},
							},
						},
					},
				}

				got := nm.GetNumEndpointsByContainerID(containerID)
				Expect(3).To(Equal(got))
			})
		})

		Context("When network does not exist", func() {
			It("Should return zero endpoints", func() {
				containerID := "989c079b-45a6-485f-8f9e-88b05d6c55c9"
				nm := &networkManager{
					ExternalInterfaces: make(map[string]*externalInterface),
				}

				got := nm.GetNumEndpointsByContainerID(containerID)
				Expect(got).To(Equal(0))
			})
		})
	})
})
