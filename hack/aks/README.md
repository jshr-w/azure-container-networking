Use this Makefile to swiftly provision/deprovision AKS clusters of different Networking flavors in Azure.

---
```bash
➜  make help
Usage:
  make <target>

Help
  help             Display this help

Utilities
  set-kubeconf     Adds the kubeconf for $CLUSTER
  unset-kubeconf   Deletes the kubeconf for $CLUSTER
  shell            print $AZCLI so it can be used outside of make

SWIFT Infra
  vars             Show the env vars configured for the swift command
  rg-up            Create resource group $GROUP in $SUB/$REGION
  rg-down          Delete the $GROUP in $SUB/$REGION
  net-up           Create required swift vnet/subnets

AKS Clusters
  byocni-up                                 Alias to swift-byocni-up
  cilium-up                                 Alias to swift-cilium-up
  up                                        Alias to swift-up
  overlay-byocni-up                         Bring up a Overlay BYO CNI cluster
  overlay-byocni-nokubeproxy-up             Bring up a Overlay BYO CNI cluster without kube-proxy
  overlay-cilium-up                         Bring up a Overlay Cilium cluster
  overlay-up                                Bring up a Overlay AzCNI cluster
  swift-byocni-up                           Bring up a SWIFT BYO CNI cluster
  swift-byocni-nokubeproxy-up               Bring up a SWIFT BYO CNI cluster without kube-proxy
  swift-cilium-up                           Bring up a SWIFT Cilium cluster
  swift-up                                  Bring up a SWIFT AzCNI cluster
  vnetscale-swift-byocni-up                 Bring up a Vnet Scale SWIFT BYO CNI cluster
  vnetscale-swift-byocni-nokubeproxy-up     Bring up a Vnet Scale SWIFT BYO CNI cluster without kube-proxy
  vnetscale-swift-cilium-up                 Bring up a Vnet Scale SWIFT Cilium cluster
  vnetscale-swift-up                        Bring up a Vnet Scale SWIFT AzCNI cluster
  windows-cniv1-up                          Bring up a Windows AzCNIv1 cluster
  linux-cniv1-up                            Bring up a Linux AzCNIv1 cluster
  dualstack-overlay-byocni-up               Bring up an dualstack overlay cluster without CNS and CNI installed
  cilium-dualstack-up                       Brings up a Cilium Dualstack Overlay cluster with Linux node only
  dualstack-byocni-nokubeproxy-up           Brings up a Dualstack overlay BYOCNI cluster with Linux node only and no kube-proxy
  windows-nodepool-up                       Add windows node pool
  down                                      Delete the cluster
  vmss-restart                              Restart the nodes of the cluster
```
