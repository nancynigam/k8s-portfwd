# k8s-portfwd
Config-driven, multi-port forwarding in Kubernetes.

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
- Create a configuration file `k8s-portfwd-config.yaml` to specify port forwarding details.

3. Build the project
```
go build
```
4. Run the executable
```
./k8s-portfwd <path-to-config>
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
Refer to the provided `k8s-portfwd-config.yaml` file for a sample configuration.

## License
MIT License
