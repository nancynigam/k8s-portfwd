# k8s-portfwd
A GoLang-based tool for multi-port forwarding in Kubernetes using a configuration file.

## Prerequisites
- Docker
- Kubectl
- Go

## Installation
1. Clone the repository
```
git clone https://github.com/nancynigam/k8s-portfwd.git
cd k8s-portfwd
```

2. Update the Kubernetes Config File
- Edit the configuration file k8s-portfwd-config to specify your port forwarding details.
- Ensure the path to your config file is correctly referenced in the main function within parseYamlFile.

3. Build the project
```
go build
```
4. Run the executable
```
./k8s-portfwd
```

## Usage
- Once the executable is running, it will start port forwarding based on the configurations provided in the config file.
- To stop port forwarding, use Ctrl + C. This will terminate all active port forwardings.

Example :
If the config contains : 
```
ports:
  - "7080:8080"
```
This configuration will forward traffic from port 8080 on your Kubernetes cluster to port 7080 on your localhost.

## Sample Configuration
Refer to the provided k8s-portfwd-config file for a sample configuration.

## License
MIT License