package main

import (
        "context"
        "fmt"
        "os"
        "path/filepath"
        "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/tools/clientcmd"
)

func main() {
        fmt.Println("Get Kubernetes pods")

        userHomeDir, err := os.UserHomeDir()
        if err != nil {
                fmt.Printf("error getting user home dir: %v\n", err)
                os.Exit(1)
        }
        kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
        fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

        kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		
        if err != nil {
                fmt.Printf("error getting Kubernetes config: %v\n", err)
                os.Exit(1)
        }

		/**
		 * In Kubernetes, a clientset is a client library provided by the Kubernetes 
		 * Go client (client-go) for interacting with the Kubernetes API server. 
		 * It is essentially a collection of client interfaces and implementations 
		 * that allow you to perform CRUD (Create, Read, Update, Delete) operations on 
		 * Kubernetes resources programmatically from within a Go application.
		 */
        clientset, err := kubernetes.NewForConfig(kubeConfig)
        if err != nil {
                fmt.Printf("error getting Kubernetes clientset: %v\n", err)
                os.Exit(1)
        }

        pods, err := clientset.CoreV1().Pods("kube-system").List(context.Background(), v1.ListOptions{})
        if err != nil {
                fmt.Printf("error getting pods: %v\n", err)
                os.Exit(1)
        }
        for _, pod := range pods.Items {
                fmt.Printf("Pod name: %s\n", pod.Name)
        }
}