# k8sfwd
Designing Config-based Kubernetes multi-port forwarding in GoLang

# Steps :
1. Install Docker, Kubectl and minikube
2. Implement port forwarding via terminal : 
    Deploy hello-minikube program - `kubectl create deployment hello-minikube --image=kicbase/echo-server:1.0`
    Expose hello-minikube program at port 8080 - `kubectl expose deployment hello-minikube --type=NodePort --port=8080`
    `minikube service hello-minikube`
    `kubectl port-forward service/hello-minikube 7080:8080`

3. 
