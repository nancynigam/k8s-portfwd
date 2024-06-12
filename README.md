# k8sfwd
Designing Config-based Kubernetes multi-port forwarding in GoLang

## Installation

Install Docker, Kubectl, and Go

## Usage

1. Clone the repo
```
git clone https://github.com/nancynigam/k8s-portfwd.git
```

2. Update k8 config file. Specify the details correctly

3. Build 
```
go build
```
4. Run the executable
```
./k8sfwd
```
5. Porwarding for all the configs mentioned in the config file will start. To stop portforwarding, use ctrl + c and it'll terminate all the port forwarding
```
Eg : ports in the config file => "7080:8080"
This implies to forward port from 8080 to localhost://7080 
```

## License
MIT License