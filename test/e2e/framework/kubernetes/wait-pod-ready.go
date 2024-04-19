package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	RetryTimeoutPodsReady  = 5 * time.Minute
	RetryIntervalPodsReady = 5 * time.Second
)

func WaitForPodReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string) error {
	podReadyMap := make(map[string]bool)

	conditionFunc := wait.ConditionWithContextFunc(func(context.Context) (bool, error) {
		// get a list of all cilium pods
		var podList *corev1.PodList
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, fmt.Errorf("error listing Pods: %w", err)
		}

		if len(podList.Items) == 0 {
			log.Printf("no pods found in namespace \"%s\" with label \"%s\"", namespace, labelSelector)
			return false, nil
		}

		// check each indviidual pod to see if it's in Running state
		for i := range podList.Items {
			var pod *corev1.Pod
			pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, podList.Items[i].Name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("error getting Pod: %w", err)
			}

			// Check the Pod phase
			if pod.Status.Phase != corev1.PodRunning {
				log.Printf("pod \"%s\" is not in Running state yet. Waiting...\n", pod.Name)
				return false, nil
			}
			if !podReadyMap[pod.Name] {
				log.Printf("pod \"%s\" is in Running state\n", pod.Name)
				podReadyMap[pod.Name] = true
			}
		}
		log.Printf("all pods in namespace \"%s\" with label \"%s\" are in Running state\n", namespace, labelSelector)
		return true, nil
	})

	// wait until all cilium pods are in Running state condition is true
	err := wait.PollUntilContextCancel(ctx, RetryIntervalPodsReady, true, conditionFunc)
	if err != nil {
		return fmt.Errorf("error waiting for pods in namespace \"%s\" with label \"%s\" to be in Running state: %w", namespace, labelSelector, err)
	}
	return nil
}
