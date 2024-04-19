package hubble

import (
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/azure-container-networking/test/e2e/framework/azure"
	"github.com/Azure/azure-container-networking/test/e2e/framework/types"
	"github.com/Azure/azure-container-networking/test/e2e/scenarios/hubble/drop"
)

const (
	// netObsRGtag is used to tag resources created by this test suite
	netObsRGtag = "-e2e-netobs-"
)

// Test against a BYO cluster with Cilium and Hubble enabled,
// create a pod with a deny all network policy and validate
// that the drop metrics are present in the prometheus endpoint
func TestE2EDropHubbleMetrics(t *testing.T) {
	job := types.NewJob("Validate that drop metrics are present in the prometheus endpoint")
	runner := types.NewRunner(t, job)
	defer runner.Run()

	curuser, _ := user.Current()

	testName := curuser.Username + netObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
	sub := os.Getenv("AZURE_SUBSCRIPTION_ID")

	job.AddStep(&azure.CreateResourceGroup{
		SubscriptionID:    sub,
		ResourceGroupName: testName,
		Location:          "eastus",
	}, nil)

	job.AddStep(&azure.CreateVNet{
		VnetName:         "testvnet",
		VnetAddressSpace: "10.0.0.0/9",
	}, nil)

	job.AddStep(&azure.CreateSubnet{
		SubnetName:         "testsubnet",
		SubnetAddressSpace: "10.0.0.0/12",
	}, nil)

	job.AddStep(&azure.CreateBYOCiliumCluster{
		ClusterName:  testName,
		PodCidr:      "10.128.0.0/9",
		DNSServiceIP: "192.168.0.10",
		ServiceCidr:  "192.168.0.0/28",
	}, nil)

	job.AddStep(&azure.GetAKSKubeConfig{
		KubeConfigFilePath: "./test.pem",
	}, nil)

	job.AddScenario(drop.ValidateDropMetric())

	job.AddStep(&azure.DeleteResourceGroup{}, nil)
}
