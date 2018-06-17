package main

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset_generated/clientset"
)

type PodMetrics struct {
	Namespace string
	Pod       string
	Container string
	CPU       string
	MEM       string
	Usage     corev1.ResourceList
}

func (p PodMetrics) UniqueID() string {
	return fmt.Sprintf("%s.%s.%s", p.Namespace, p.Pod, p.Container)
}

type KubeMetrics struct {
	namespace     string
	metricsClient *metricsclientset.Clientset

	mu      sync.Mutex
	metrics []PodMetrics
}

func (k *KubeMetrics) GetMetrics() []PodMetrics {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.metrics
}

func (k *KubeMetrics) FetchMetrics() error {
	k.mu.Lock()
	defer k.mu.Unlock()

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
			k.metrics = append(k.metrics, pr)
		}
	}
	return nil
}
