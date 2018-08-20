package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	kubeconfig   string
	incluster    bool
	svcWebhook   string
	podWebhook   string
	eventWebhook string
	interval     int
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flag.BoolVar(&incluster, "incluster", false, "use incluster config")
	flag.StringVar(&svcWebhook, "svcWebhook", "", "webhook for svc")
	flag.StringVar(&podWebhook, "podWebhook", "", "webhook for pod")
	flag.StringVar(&eventWebhook, "eventWebhook", "", "webhook for event")
	flag.IntVar(&interval, "interval", 900, "interval in seconds to scan")
}

func main() {
	var (
		config *rest.Config
		err    error
	)

	flag.Parse()
	log.Printf("incluster: %v\n", incluster)
	log.Printf("svcWebhook: %s\n", svcWebhook)
	log.Printf("podWebhook: %s\n", podWebhook)
	log.Printf("eventWebhook: %s\n", eventWebhook)

	if incluster {
		log.Println("use in cluster config")
		config, err = rest.InClusterConfig()
	} else {
		log.Println("kubeocnfig is", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		log.Panicf("unable to get cluster config. err: %v", err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicf("unable to get k8s client. err: %v", err)
	}

	c := make(chan os.Signal, 1)
	quit := make(chan struct{})
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("exiting...")
		close(quit)
	}()

	go func() {
		for {
			log.Println("Do...")
			Do(clientset)
			log.Printf("Sleep %d seconds\n", interval)
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()

	<-quit
	log.Println("existed")
}

func Do(clientset *kubernetes.Clientset) {
	svcList, err := clientset.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		log.Panicf("unable to list svc. err: %v", err)
	}

	podList, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		log.Panicf("unable to list pod. err: %v", err)
	}

	eventList, err := clientset.CoreV1().Events("").List(metav1.ListOptions{})
	if err != nil {
		log.Panicf("unable to list events. err: %v", err)
	}

	log.Println("post to svc webhook")
	if svcWebhook != "" {
		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(svcList)
		resp, err := http.Post(svcWebhook, "application/json", b)
		if err != nil {
			log.Panicf("unable to post webhook. err: %v", err)
		}
		log.Println("status code:", resp.StatusCode)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}

	log.Println("post to pod webhook")
	if podWebhook != "" {
		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(podList)
		resp, err := http.Post(podWebhook, "application/json", b)
		if err != nil {
			log.Panicf("unable to post webhook. err: %v", err)
		}
		log.Println("status code:", resp.StatusCode)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}

	log.Println("post to event webhook")
	if eventWebhook != "" {
		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(eventList)
		resp, err := http.Post(eventWebhook, "application/json", b)
		if err != nil {
			log.Panicf("unable to post webhook. err: %v", err)
		}
		log.Println("status code:", resp.StatusCode)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}
}
