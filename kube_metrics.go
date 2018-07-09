package main

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsclientset "k8s.io/metrics/pkg/client/clientset_generated/clientset"
)

type PodMetrics struct {
	Namespace        string
	Pod              string
	Container        string
	Node             string
	CPU              string
	MEM              string
	Usage            corev1.ResourceList
	ResourceRequests corev1.ResourceList
	ResourceLimits   corev1.ResourceList
}

func (p PodMetrics) UniqueID() string {
	return fmt.Sprintf("%s.%s.%s", p.Namespace, p.Pod, p.Container)
}

func (p PodMetrics) InfoString() string {
	return fmt.Sprintf("requests: %s -- limits: %s", p.formatResource(p.ResourceRequests), p.formatResource(p.ResourceLimits))
}

func (p PodMetrics) formatResource(rl corev1.ResourceList) string {
	return fmt.Sprintf("cpu=%s mem=%dMi", rl.Cpu().String(), rl.Memory().ScaledValue(resource.Mega))

}

type KubeMetrics struct {
	namespace     string
	metricsClient *metricsclientset.Clientset
	kubeClient    *kubernetes.Clientset

	mu      sync.Mutex
	metrics []PodMetrics

	fetchedResources bool
	resources        map[string]PodMetrics
}

func (k *KubeMetrics) GetMetrics() []PodMetrics {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.metrics
}

func (k *KubeMetrics) FetchMetrics() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := k.FetchResources(); err != nil {
		return err
	}

	metrics, err := k.metricsClient.Metrics().PodMetricses(k.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get pod metrics")
	}

	k.metrics = []PodMetrics{}
	for _, pod := range metrics.Items {
		for _, c := range pod.Containers {
			pr := PodMetrics{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
				Container: c.Name,
				CPU:       c.Usage.Cpu().String(),
				MEM:       fmt.Sprintf("%dMi", c.Usage.Memory().ScaledValue(resource.Mega)),
				Usage:     c.Usage,
			}
			if resources, ok := k.resources[pr.UniqueID()]; ok {
				pr.ResourceRequests = resources.ResourceRequests
				pr.ResourceLimits = resources.ResourceLimits
			}
			k.metrics = append(k.metrics, pr)
		}
	}
	return nil
}

func (k *KubeMetrics) FetchResources() error {
	if k.fetchedResources {
		return nil
	}

	pods, err := k.kubeClient.CoreV1().Pods(k.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get pod resources")
	}

	podMetrics := make(map[string]PodMetrics)

	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			pr := PodMetrics{
				Pod:              pod.Name,
				Node:             pod.Spec.NodeName,
				Namespace:        pod.Namespace,
				Container:        c.Name,
				ResourceRequests: c.Resources.Requests,
				ResourceLimits:   c.Resources.Limits,
			}
			podMetrics[pr.UniqueID()] = pr
		}
	}
	k.resources = podMetrics
	k.fetchedResources = true
	return nil
}
