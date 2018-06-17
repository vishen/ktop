package main

import (
	"log"
	"os"
	"time"

	termbox "github.com/nsf/termbox-go"

	// Kubernetes
	"k8s.io/client-go/tools/clientcmd"

	// Kubernetes metrics
	metricsclientset "k8s.io/metrics/pkg/client/clientset_generated/clientset"

	// GKE authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	watchSeconds = 5
)

var (
	kubeMetrics KubeMetrics
)

func main() {
	kubeConfig := ""
	kubeContext := ""

	// Determine kubeconfig path
	if kubeConfig == "" {
		if os.Getenv("KUBECONFIG") != "" {
			kubeConfig = os.Getenv("KUBECONFIG")
		} else {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}
	// Create the kubernetes client configuration
	clientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: kubeConfig,
		},
		&clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
		},
	).ClientConfig()
	if err != nil {
		log.Fatalf("unable to create k8s client config: %s\n", err)
	}

	log.Printf("connecting to kubernetes cluster metrics\n")
	metricsClient, err := metricsclientset.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("unable to create metrics client: %s\n", err)
	}

	kubeMetrics = KubeMetrics{metricsClient: metricsClient}
	if err := kubeMetrics.FetchMetrics(); err != nil {
		log.Fatalf("unable to get kubernetes metrics: %s", err)
	}

	if err := termbox.Init(); err != nil {
		log.Fatalf("error init termbox: %s", err)
	}
	defer termbox.Close()

	termWidth, termHeight = termbox.Size()

	termbox.SetInputMode(termbox.InputEsc | termbox.InputAlt | termbox.InputMouse)

	go func() {
		updateScreen()
		for range time.NewTicker(time.Second * watchSeconds).C {
			kubeMetrics.FetchMetrics()
			updateScreen()
		}
	}()

	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventMouse:
			setMouseClick(ev.MouseX, ev.MouseY, ev.Key)
			updateScreen()
		case termbox.EventResize:
			termWidth, termHeight = ev.Width, ev.Height
			updateScreen()
		case termbox.EventKey:
			if ev.Key == termbox.KeyEsc {
				return
			} else if ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2 {
				if len(filterString) > 0 {
					filterString = filterString[:len(filterString)-1]
				}
			}
			switch ch := ev.Ch; ch {
			case '1': // key 1
				setOrderOption(OrderCPUDec)
			case '2': // key 2
				setOrderOption(OrderCPUAsc)
			case '3': // key 3
				setOrderOption(OrderMEMDec)
			case '4': // key 4
				setOrderOption(OrderMEMAsc)
			default:
				if (ch >= 'a' && ch <= 'z') || ch == '-' || ch == '_' {
					filterString += string(ch)
				}
			}
			updateScreen()
		}
	}

}
