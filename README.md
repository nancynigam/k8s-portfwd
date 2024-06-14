# k8s-portfwd
Designing Config-based Kubernetes multi-port forwarding in GoLang

## Installation

Install Docker, Kubectl, and Go

## Usage

1. Clone the repo
```
git clone https://github.com/nancynigam/k8s-portfwd.git
```

2. Update k8s config file. Specify the details correctly. A sample config file
k8s-portfwd-config is provided for reference.

3. Update the name/path of config file in main inside parseYamlFile function.

4. Build 
```
go build
```
5. Run the executable
```
./k8s-portfwd
```
6. Porwarding for all the configs mentioned in the config file will start. To stop portforwarding, use ctrl + c and it'll terminate all the port forwarding
```
Eg : ports in the config file => "7080:8080"
This implies to forward port from 8080 to localhost://7080 
```

## License
MIT License