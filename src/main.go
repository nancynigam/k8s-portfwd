package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Define the structure based on the YAML format
type Config struct {
	// This is public field as it starts with an uppercase
	// yaml:"version": This is a struct tag. Struct tags are a way to attach
	// meta-information to struct fields. The yaml:"version" tag indicates that this
	// field should be associated with the key version when the struct is serialized to
	// or deserialized from YAML.
	Version string `yaml:"version"`
	Config  struct {
		RetryDelaySec float64 `yaml:"retry_delay_sec"`
	} `yaml:"config"`
	Targets []Target `yaml:"targets"`
}

type Target struct {
	Name        string   `yaml:"name,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Target      string   `yaml:"target"`
	Type        string   `yaml:"type"`
	Namespace   string   `yaml:"namespace"`
	ListenAddrs []string `yaml:"listen_addrs,omitempty"`
	Ports       []string `yaml:"ports"`
}

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

	// Defer a function call to handle panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
			// Optionally, perform cleanup or log the panic
		}
	}()

	// synchronization primitive used to wait for a collection of goroutines to finish
	// executing. This is to ensure all parts of a concurrent program have completed
	// before exiting.
	var wg sync.WaitGroup

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
	//fmt.Printf(" Created stop Channel %v\n", stopCh)

	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	//fmt.Printf(" Created stream %v\n", stream)

	// manage termination signal from the terminal. As you can see the stopCh gets closed
	// to gracefully handle its termination.
	// Sigs channel is created to receive OS signals.
	sigs := make(chan os.Signal, 1)
	//fmt.Printf(" Created sig var %v\n", sigs)

	// This line sets up a signal notification system that allows your program to catch and
	// handle specific signals, such as interruptions and terminations, instead of terminating abruptly.
	// 'signal.Notify' relays incoming signals from the OS to a Go channel
	// It registers sigs channel to receive notifications from syscall.SIGINT (Cltr+C), syscall.SIGTERM (kill)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	wg.Add(1)
	go handleStopSignal(stopCh, sigs, &wg)

	var yamlConfig Config = parseYamlFile()

	for _, target := range yamlConfig.Targets {

		// readyCh communicate when the port forward is ready to get traffic
		readyCh := make(chan struct{})
		//fmt.Printf(" Created ready Channel %v\n", readyCh)
		podname := target.Target
		namespace := target.Namespace
		ports := strings.Split(target.Ports[0], ":")
		localTargetPort, err := strconv.Atoi(ports[0])

		// TODO : Add error handling for both
		if err != nil {
			panic(err)
		}

		podPort, err := strconv.Atoi(ports[1])
		if err != nil {
			panic(err)
		}

		fmt.Printf("Target podname: %s\n", podname)
		fmt.Printf("Target namespace: %s\n", namespace)
		fmt.Printf("Target localTargetPort: %d\n", localTargetPort)
		fmt.Printf("Target podPort: %d\n", podPort)

		go startPortForwarding(PortForwardAPodRequest{
			RestConfig: kubeConfig,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podname,
					Namespace: namespace,
				},
			},
			LocalPort: localTargetPort,
			PodPort:   podPort,
			Streams:   stream,
			StopCh:    stopCh,
			ReadyCh:   readyCh,
		}, &wg)

		// Once the port forwarding operation is initiated, the goroutine waits for a
		// signal indicating that the operation is ready to proceed further.
		// The PortForwardAPod function, upon successfully setting up the port forwarding,
		// signals readiness by sending a value (e.g., true) over the readyCh channel.
		select {
		case <-readyCh:
			println("port is ready to get traffic\n")
		}
	}
	// The wg.Wait() is used to ensure that all goroutines managed by the sync.WaitGroup
	// have completed their execution before the main goroutine exits.
	// Puttinh it at the end of main.
	wg.Wait()
}

// Starting first go routine to handle signals
func handleStopSignal(stopCh chan struct{}, sigs chan os.Signal, wg *sync.WaitGroup) {

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

}

func startPortForwarding(req PortForwardAPodRequest, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("Entered PortForwarder goroutine\n")
	if err := PortForwardAPod(req); err != nil {
		panic(err)
	}
}

/**
 * SPDY (pronounced "speedy") is a network protocol developed primarily at Google for
 * transporting web content. It was designed to improve the performance and security of
 * web browsing by reducing latency through compression, multiplexing, and prioritization of web requests.
 */
func PortForwardAPod(req PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", req.Pod.Namespace, req.Pod.Name)
	// fmt.Println(" Path : " + path)
	// fmt.Println()

	// Minikube creates a local Kubernetes cluster by setting up a VM or container
	// and installing the necessary Kubernetes components. It configures the Kubernetes API server
	//to be accessible via localhost using port forwarding. This setup allows you to interact with your local
	// cluster as if it were running directly on your machine, providing a convenient environment for development and testing.
	hostIP := strings.TrimLeft(req.RestConfig.Host, "https:/")
	// fmt.Println(" hostIP : " + hostIP)
	// fmt.Println()

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		// return err
		panic(err)
	}

	// New Dialer creates a dialer which connects to the provided URL and upgrades connected to SPDY
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	// Using portforward API from client-go; New creates a new PortForwarder with localhost listen addresses
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)

	if err != nil {
		//return err
		panic(err)
	}

	// ForwardPorts formats and executes a port forwarding request. The connection will remain
	// open until stopChan is closed.
	return fw.ForwardPorts()
}

func parseYamlFile() Config {

	data, err := os.ReadFile("../k8sfwd-example.yaml")
	if err != nil {
		//log.Fatalf("error: %v", err)
		panic(err)
	}

	// Parse the YAML file into the Config struct
	var config Config
	// The yaml.Unmarshal function is used to parse the YAML data and populate the fields of the struct.
	// The Unmarshal function takes two arguments:
	// The byte slice containing the YAML data (data).
	// A pointer to the struct where the parsed data should be stored (&config).
	// The Unmarshal function signature looks like this:
	// When you call yaml.Unmarshal(data, &config), the function parses the YAML data in
	// data and fills the config struct with the corresponding values from the YAML.
	err = yaml.Unmarshal(data, &config)

	if err != nil {
		//log.Fatalf("error: %v", err)
		panic(err)
	}

	return config
}
