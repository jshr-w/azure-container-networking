//go:build load

package load

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-container-networking/test/internal/kubernetes"
	"github.com/Azure/azure-container-networking/test/validate"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	OSType            string `env:"OS_TYPE" default:"linux"`
	CNIType           string `env:"CNI_TYPE" default:"cilium"`
	Iterations        int    `env:"ITERATIONS" default:"2"`
	ScaleUpReplicas   int    `env:"SCALE_UP" default:"10"`
	ScaleDownReplicas int    `env:"SCALE_DOWN" default:"1"`
	Replicas          int    `env:"REPLICAS" default:"1"`
	ValidateStateFile bool   `env:"VALIDATE_STATEFILE" default:"false"`
	ValidateDualStack bool   `env:"VALIDATE_DUALSTACK" default:"false"`
	ValidateV4Overlay bool   `env:"VALIDATE_V4OVERLAY" default:"false"`
	SkipWait          bool   `env:"SKIP_WAIT" default:"false"`
	RestartCase       bool   `env:"RESTART_CASE" default:"false"`
	Cleanup           bool   `env:"CLEANUP" default:"false"`
	CNSOnly           bool   `env:"CNS_ONLY" default:"false"`
}

const (
	manifestDir      = "../manifests"
	podLabelSelector = "load-test=true"
	namespace        = "load-test"
)

var testConfig = &TestConfig{}

var noopDeploymentMap = map[string]string{
	"windows": manifestDir + "/noop-deployment-windows.yaml",
	"linux":   manifestDir + "/noop-deployment-linux.yaml",
}

// This map is used exclusively for TestLoad. Windows is expected to take 10-15 minutes per iteration.
// Will change this as scale testing results are verified. This will ensure we keep a standard performance metric.
var scaleTimeoutMap = map[string]time.Duration{
	"windows": 15 * time.Minute,
	"linux":   10 * time.Minute,
}

/*
In order to run the scale tests, you need a k8s cluster and its kubeconfig.
If no kubeconfig is passed, the test will attempt to find one in the default location for kubectl config.
Run the tests as follows:

go test -timeout 30m -tags load -run ^TestLoad$

The Load test scale the pods up/down on the cluster and validates the pods have IP. By default it runs the
cycle for 2 iterations.

To validate the state file, set the flag -validate-statefile to true. By default it is set to false.
todo: consider adding the following scenarios
- [x] All pods should be assigned an IP.
- [x] Test the CNS state file.
- [x] Test the CNS Local cache.
- [x] Test the Cilium state file.
- [x] Test the Node restart.
- [x] Test based on operating system.
- [x] Test the HNS state file.
- [x] Parameterize the os, cni and number of iterations.
- [x] Add deployment yaml for windows.
*/
func TestLoad(t *testing.T) {
	clientset := kubernetes.MustGetClientset()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(testConfig.Iterations)*scaleTimeoutMap[testConfig.OSType])
	defer cancel()

	// Create namespace if it doesn't exist
	namespaceExists, err := kubernetes.NamespaceExists(ctx, clientset, namespace)
	require.NoError(t, err)

	if !namespaceExists {
		kubernetes.MustCreateNamespace(ctx, clientset, namespace)
	}

	deployment := kubernetes.MustParseDeployment(noopDeploymentMap[testConfig.OSType])
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	kubernetes.MustCreateDeployment(ctx, deploymentsClient, deployment)

	t.Log("Checking pods are running")
	err = kubernetes.WaitForPodsRunning(ctx, clientset, namespace, podLabelSelector)
	require.NoError(t, err)

	t.Log("Repeating the scale up/down cycle")
	for i := 0; i < testConfig.Iterations; i++ {
		t.Log("Iteration ", i)
		t.Log("Scale down deployment")
		kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, testConfig.ScaleDownReplicas, testConfig.SkipWait)

		t.Log("Scale up deployment")
		kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, testConfig.ScaleUpReplicas, testConfig.SkipWait)
	}
	t.Log("Checking pods are running and IP assigned")
	err = kubernetes.WaitForPodsRunning(ctx, clientset, "", "")
	require.NoError(t, err)

	if testConfig.ValidateStateFile {
		t.Run("Validate state file", TestValidateState)
	}

	if testConfig.ValidateV4Overlay {
		t.Run("Validate v4overlay", TestV4OverlayProperties)
	}

	if testConfig.ValidateDualStack {
		t.Run("Validate dualstack overlay", TestDualStackProperties)
	}

	if testConfig.Cleanup {
		kubernetes.MustDeleteDeployment(ctx, deploymentsClient, deployment)
		err = kubernetes.WaitForPodsDelete(ctx, clientset, namespace, podLabelSelector)
		require.NoError(t, err, "error waiting for pods to delete")
	}
}

// TestValidateState validates the state file based on the os and cni type.
func TestValidateState(t *testing.T) {
	clientset := kubernetes.MustGetClientset()

	config := kubernetes.MustGetRestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if testConfig.ValidateStateFile {
		deployment := kubernetes.MustParseDeployment(noopDeploymentMap[testConfig.OSType])
		deploymentsClient := clientset.AppsV1().Deployments(namespace)

		// Ensure pods exist on nodes to validate state files properly. Can obtain false positives without pods.
		nodes, err := kubernetes.GetNodeListByLabelSelector(ctx, clientset, "kubernetes.io/os="+testConfig.OSType)
		require.NoError(t, err)
		nodeCount := len(nodes.Items)
		replicas := int32(nodeCount) * 2

		deploymentExists, err := kubernetes.DeploymentExists(ctx, deploymentsClient, deployment.Name)
		require.NoError(t, err)
		if !deploymentExists {
			t.Logf("Test deployment %s does not exist! Create %v pods in %s namespace", deployment.Name, replicas, namespace)
			// Create namespace if it doesn't exist
			namespaceExists, err := kubernetes.NamespaceExists(ctx, clientset, namespace)
			require.NoError(t, err)
			if !namespaceExists {
				kubernetes.MustCreateNamespace(ctx, clientset, namespace)
			}

			kubernetes.MustCreateDeployment(ctx, deploymentsClient, deployment)
			kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, int(replicas), false)
		} else {
			t.Log("Test deployment exists! Use existing setup")
			replicas, err = kubernetes.GetDeploymentAvailableReplicas(ctx, deploymentsClient, deployment.Name) // If test namespace exists then use existing Replicas
			if replicas != 0 && err != nil {
				require.NoError(t, err)
			}
		}
		if replicas < int32(nodeCount) {
			t.Logf("Warning - current replica count %v is below current %s node count of %d. Raising replicas to minimum required to ensure there is a pod on every node.", replicas, testConfig.OSType, nodeCount)
			replicas = int32(nodeCount * 2)
			kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, int(replicas), false)
		}
		t.Log("Ensure deployment is in ready status")
		err = kubernetes.WaitForPodDeployment(ctx, clientset, namespace, deployment.Name, podLabelSelector, int(replicas))
		require.NoError(t, err)
	}

	validator, err := validate.CreateValidator(ctx, clientset, config, namespace, testConfig.CNIType, testConfig.RestartCase, testConfig.OSType)
	require.NoError(t, err)

	err = validator.Validate(ctx)
	require.NoError(t, err)

	if testConfig.Cleanup {
		validator.Cleanup(ctx)
	}
}

// TestScaleDeployment scales the deployment up/down based on the replicas passed.
// REPLICAS=10 go test -timeout 30m -tags load -run ^TestScaleDeployment$ -tags=load
func TestScaleDeployment(t *testing.T) {
	t.Log("Scale deployment")
	clientset := kubernetes.MustGetClientset()

	ctx := context.Background()
	// Create namespace if it doesn't exist
	namespaceExists, err := kubernetes.NamespaceExists(ctx, clientset, namespace)
	require.NoError(t, err)

	if !namespaceExists {
		kubernetes.MustCreateNamespace(ctx, clientset, namespace)
	}

	deployment := kubernetes.MustParseDeployment(noopDeploymentMap[testConfig.OSType])

	if testConfig.Cleanup {
		deploymentsClient := clientset.AppsV1().Deployments(namespace)
		kubernetes.MustCreateDeployment(ctx, deploymentsClient, deployment)
	}

	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, testConfig.Replicas, testConfig.SkipWait)

	if testConfig.Cleanup {
		kubernetes.MustDeleteDeployment(ctx, deploymentsClient, deployment)
		err = kubernetes.WaitForPodsDelete(ctx, clientset, namespace, podLabelSelector)
		require.NoError(t, err, "error waiting for pods to delete")
	}
}

// TestValidCNSStateDuringScaleAndCNSRestartToTriggerDropgzInstall
// tests that dropgz install during a pod scaling event, does not crash cns
func TestValidCNSStateDuringScaleAndCNSRestartToTriggerDropgzInstall(t *testing.T) {
	clientset := kubernetes.MustGetClientset()

	config := kubernetes.MustGetRestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Provide an option to validate state files with a proper environment before running test
	if testConfig.ValidateStateFile {
		t.Run("Validate state file", TestValidateState)
	}

	validator, err := validate.CreateValidator(ctx, clientset, config, namespace, testConfig.CNIType, testConfig.RestartCase, testConfig.OSType)
	require.NoError(t, err)

	deployment := kubernetes.MustParseDeployment(noopDeploymentMap[testConfig.OSType])
	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	if testConfig.Cleanup {
		// Create namespace if it doesn't exist
		namespaceExists, err := kubernetes.NamespaceExists(ctx, clientset, namespace)
		require.NoError(t, err)
		if !namespaceExists {
			kubernetes.MustCreateNamespace(ctx, clientset, namespace)
		}

		// Create a deployment
		kubernetes.MustCreateDeployment(ctx, deploymentsClient, deployment)
	}

	// Scale it up and "skipWait", so CNS restart can happen immediately after scale call is made (while pods are still creating)
	skipWait := true
	kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, testConfig.ScaleUpReplicas, skipWait)

	// restart linux CNS (linux, windows)
	err = kubernetes.RestartCNSDaemonset(ctx, clientset, true)
	require.NoError(t, err)

	// wait for pods to settle before checking cns state (otherwise, race between getting pods in creating state, and getting CNS state file)
	err = kubernetes.WaitForPodDeployment(ctx, clientset, namespace, deployment.Name, podLabelSelector, testConfig.ScaleUpReplicas)
	require.NoError(t, err)

	// Validate the CNS state
	err = validator.Validate(ctx)
	require.NoError(t, err)

	// Scale it down
	kubernetes.MustScaleDeployment(ctx, deploymentsClient, deployment, clientset, namespace, podLabelSelector, testConfig.ScaleDownReplicas, skipWait)

	// restart linux CNS (linux, windows)
	err = kubernetes.RestartCNSDaemonset(ctx, clientset, true)
	require.NoError(t, err)

	// wait for pods to settle before checking cns state (otherwise, race between getting pods in terminating state, and getting CNS state file)
	err = kubernetes.WaitForPodDeployment(ctx, clientset, namespace, deployment.Name, podLabelSelector, testConfig.ScaleDownReplicas)
	require.NoError(t, err)

	// Validate the CNS state
	err = validator.Validate(ctx)
	require.NoError(t, err)

	if testConfig.Cleanup {
		kubernetes.MustDeleteDeployment(ctx, deploymentsClient, deployment)
		err = kubernetes.WaitForPodsDelete(ctx, clientset, namespace, podLabelSelector)
		require.NoError(t, err, "error waiting for pods to delete")
		validator.Cleanup(ctx)
	}
}

func TestV4OverlayProperties(t *testing.T) {
	if !testConfig.ValidateV4Overlay {
		return
	}
	clientset := kubernetes.MustGetClientset()

	config := kubernetes.MustGetRestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	validator, err := validate.CreateValidator(ctx, clientset, config, namespace, testConfig.CNIType, testConfig.RestartCase, testConfig.OSType)
	require.NoError(t, err)

	// validate IPv4 overlay scenarios
	t.Log("Validating v4Overlay node labels")
	err = validator.ValidateV4OverlayControlPlane(ctx)
	require.NoError(t, err)

	if testConfig.Cleanup {
		validator.Cleanup(ctx)
	}
}

func TestDualStackProperties(t *testing.T) {
	if !testConfig.ValidateDualStack {
		return
	}
	clientset := kubernetes.MustGetClientset()

	config := kubernetes.MustGetRestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	t.Log("Validating the dualstack node labels")
	validator, err := validate.CreateValidator(ctx, clientset, config, namespace, testConfig.CNIType, testConfig.RestartCase, testConfig.OSType)
	require.NoError(t, err)

	// validate dualstack overlay scenarios
	err = validator.ValidateDualStackControlPlane(ctx)
	require.NoError(t, err)

	if testConfig.Cleanup {
		validator.Cleanup(ctx)
	}
}
