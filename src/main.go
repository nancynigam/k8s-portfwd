package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Define the structure based on the YAML format
type Config struct {
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
	Cluster     string   `yaml:"cluster"`
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
		}
	}()

	var wg sync.WaitGroup

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

	//kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

	pathToFile := os.Args[1]

	/** stopCh control the port forwarding lifecycle.
	 * When it gets closed the port forward will terminate
	 * This is used to stop the main function from terminating
	 * Till value is received in this channel
	 */
	stopCh := make(chan struct{}, 1)

	/**
	 * stream is used to tell the port forwarder where to place its output or
	 * where to expect input if needed. For the port forwarding we just need
	 * the output eventually
	 */
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// manage termination signal from the terminal. Sigs channel is created to receive OS signals.
	sigs := make(chan os.Signal, 1)

	/**
	 * This line sets up a signal notification system that allows your program to catch and
	 * handle specific signals, such as interruptions and terminations, instead of terminating abruptly.
	 * 'signal.Notify' relays incoming signals from the OS to a Go channel
	 * It registers sigs channel to receive notifications from syscall.SIGINT (Cltr+C), syscall.SIGTERM (kill)
	 * signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	 */
	wg.Add(1)

	go handleStopSignal(stopCh, sigs, &wg)

	var yamlConfig Config = parseYamlFile(pathToFile)

	for _, target := range yamlConfig.Targets {

		// readyCh communicate when the port forward is ready to get traffic
		readyCh := make(chan struct{})
		podname := target.Target
		namespace := target.Namespace
		cluster := target.Cluster
		ports := strings.Split(target.Ports[0], ":")
		localTargetPort, err := strconv.Atoi(ports[0])

		if err != nil {
			panic(err)
		}

		podPort, err := strconv.Atoi(ports[1])
		if err != nil {
			panic(err)
		}

		// Update the current context to config cluster
		if cluster != "" {

			config, err := loadKubeConfig(kubeConfigPath)
			if err != nil {
				log.Fatalf("Failed to load kubeconfig: %v", err)
			}

			err = updateCurrentContext(config, cluster)
			if err != nil {
				log.Fatalf("Failed to update current context: %v", err)
			}

			// Save the updated kubeconfig back to the file
			err = saveKubeConfig(kubeConfigPath, config)
			if err != nil {
				log.Fatalf("Failed to save updated kubeconfig: %v", err)
			}

			fmt.Printf("Successfully updated the current context to %s\n", cluster)

		}

		kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

		if err != nil {
			fmt.Printf("error getting Kubernetes config: %v\n", err)
			os.Exit(1)
		}

		if len(os.Args) < 2 {
			fmt.Println("Usage: ./executable filename.txt")
			os.Exit(1)
		}

		fmt.Printf("Target cluster: %s\n", cluster)
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

		/**
		 * Once the port forwarding operation is initiated, the goroutine waits for a
		 * signal indicating that the operation is ready to proceed further.
		 * The PortForwardAPod function, upon successfully setting up the port forwarding,
		 * signals readiness by sending a value (e.g., true) over the readyCh channel.
		 */
		select {
		case <-readyCh:
			println("port is ready to get traffic\n")
		}
	}
	// Ensures that all the goroutines have completed their execution before the main goroutine exits.
	wg.Wait()
}

// Starting first go routine to handle signals
func handleStopSignal(stopCh chan struct{}, sigs chan os.Signal, wg *sync.WaitGroup) {

	// Goroutine blocks here, it's waiting to receive signal from the sigs channel.
	<-sigs

	// Once the signal is received, it prints the following message to indicate program is about to shut down
	fmt.Println("Connection interrupted ..")

	// StopCh is used to signal other parts of program to stop their work & cleanup.
	// Closing a channel is the best way to broadcast a signal to all goroutines that are waiting on it.
	close(stopCh)

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

	hostIP := strings.TrimLeft(req.RestConfig.Host, "https:/")

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

	// ForwardPorts formats and executes a port forwarding request. The connection will remain open until stopChan is closed.
	return fw.ForwardPorts()
}

func parseYamlFile(pathToFile string) Config {

	data, err := os.ReadFile(pathToFile)
	if err != nil {
		//log.Fatalf("error: %v", err)
		panic(err)
	}

	// Parse the YAML file into the Config struct
	var config Config

	err = yaml.Unmarshal(data, &config)

	if err != nil {
		//log.Fatalf("error: %v", err)
		panic(err)
	}

	return config
}

// updateCurrentContext updates the current context in the kubeconfig
func updateCurrentContext(config *api.Config, newContext string) error {
	// Check if the new context exists in the kubeconfig
	if _, exists := config.Contexts[newContext]; !exists {
		return fmt.Errorf("context %s not found in kubeconfig", newContext)
	}

	// Update the current context
	config.CurrentContext = newContext
	return nil
}

// saveKubeConfig writes the updated kubeconfig back to the specified file
func saveKubeConfig(kubeconfigPath string, config *api.Config) error {
	// Serialize the updated config
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failed to serialize updated kubeconfig: %v", err)
	}

	// Write the updated config to the file
	err = os.WriteFile(kubeconfigPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated kubeconfig: %v", err)
	}

	return nil
}

// loadKubeConfig loads the kubeconfig from the specified path
func loadKubeConfig(kubeconfigPath string) (*api.Config, error) {
	// Read the kubeconfig file
	kubeconfigBytes, err := ioutil.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig: %v", err)
	}

	// Load the kubeconfig into a Config object
	config, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	return config, nil
}
