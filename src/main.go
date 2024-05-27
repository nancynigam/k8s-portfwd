package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwardAPodRequest struct {

	// RestConfig is the kubernetes config
	RestConfig *rest.Config

	// Pod selected for port forwarding
	Pod v1.Pod

	// LocalPort that will be selected to expose the PodPort
	LocalPort int

	// TargtePod for the pod
	PodPort int

	// Streams configure where to read or write input from
	Streams genericclioptions.IOStreams

	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}

	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

func main() {
	// synchronization primitive used to wait for a collection of goroutines to finish
	// executing. This is to ensure all parts of a concurrent program have completed
	// before exiting.
	var wg sync.WaitGroup
	wg.Add(1)
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)
	// This line constructs a configuration object from the kubeconfig file,
	// enabling the program to authenticate and interact with the Kubernetes cluster.
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	// fmt.Printf(" kubeconfig: %s\n", kubeConfig)

	if err != nil {
		fmt.Printf("error getting Kubernetes config: %v\n", err)
		os.Exit(1)
	}

	// stopCh control the port forwarding lifecycle.
	// When it gets closed the port forward will terminate
	// This is used to stop the main function from terminating
	// Till value is received in this channel
	stopCh := make(chan struct{}, 1)
	fmt.Printf(" Created stop Channel %v\n", stopCh)

	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	fmt.Printf(" Created ready Channel %v\n", readyCh)

	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	fmt.Printf(" Created stream %v\n", stream)

	// manage termination signal from the terminal. As you can see the stopCh gets closed
	// to gracefully handle its termination.
	// Sigs channel is created to receive OS signals.
	sigs := make(chan os.Signal, 1)
	fmt.Printf(" Created sig var %v\n", sigs)

	// This line sets up a signal notification system that allows your program to catch and
	// handle specific signals, such as interruptions and terminations, instead of terminating abruptly.
	// 'signal.Notify' relays incoming signals from the OS to a Go channel
	// It registers sigs channel to receive notifications from syscall.SIGINT (Cltr+C), syscall.SIGTERM (kill)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Starting first go routine to handle signals
	go func() {

		// Goroutine blocks here, it's waiting to receive signal from the sigs channel.
		// This goroutine only proceeds when signal is received
		<-sigs

		// Once the signal is received, it prints "Bye" to indicate program is about to shut down
		fmt.Println("Bye ..")

		// StopCh is used to signal other parts of program to stop their work & cleanup.
		// Closing a channel is the best way to broadcast a signal to all goroutines that are waiting on it.
		close(stopCh)

		// indicates gorutine is finished, decrementing waitroup counter. This ensures that the main function
		// can wait for all goroutines to finish their cleanup before exiting.
		wg.Done()

	}() // instantly call the func, don't have to wait for func call

	go func() {
		fmt.Printf(" Entered PortForwarder goroutine\n")
		// PortForward the pod specified from its port 8080 to the local port 7070
		err := PortForwardAPod(PortForwardAPodRequest{
			RestConfig: kubeConfig,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-minikube-5c898d8489-7mzk5",
					Namespace: "default",
				},
			},
			LocalPort: 7070,
			PodPort:   8080,
			Streams:   stream,
			StopCh:    stopCh,
			ReadyCh:   readyCh,
		})

		if err != nil {
			panic(err)
		}
	}()

	select {
	case <-readyCh:
		break
	}
	println("port is ready to get traffic")

	wg.Wait()
}

/**
 * SPDY (pronounced "speedy") is a network protocol developed primarily at Google for
 * transporting web content. It was designed to improve the performance and security of
 * web browsing by reducing latency through compression, multiplexing, and prioritization of web requests.
 */
func PortForwardAPod(req PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", req.Pod.Namespace, req.Pod.Name)
	fmt.Println(" Path : " + path)
	fmt.Println()

	hostIP := strings.TrimLeft(req.RestConfig.Host, "https:/")
	fmt.Println(" hostIP : " + hostIP)
	fmt.Println()

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	// New Dialer creates a dialer which connects to the provided URL and upgrades connected to SPDY
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	// Using portforward API from client-go; New creates a new PortForwarder with localhost listen addresses
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)

	if err != nil {
		return err
	}

	// ForwardPorts formats and executes a port forwarding request. The connection will remain
	// open until stopChan is closed.
	return fw.ForwardPorts()
}
