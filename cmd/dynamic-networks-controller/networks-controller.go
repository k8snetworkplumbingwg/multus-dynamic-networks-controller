package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1coreinformerfactory "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	nadinformers "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/informers/externalversions"

	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/config"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/controller"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/cri"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/logging"
	"github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller/pkg/multuscni"
)

const (
	ErrorLoadingConfig int = iota + 1
	ErrorBuildingController
)

func main() {
	klog.InitFlags(nil)
	configFilePath := flag.String(
		"config",
		config.DefaultDynamicNetworksControllerConfigFile,
		"Specify the path to the multus-daemon configuration")

	flag.Parse()

	klog.Infof("dynamic-networks-controller: built from [%s]", controllerVersion())

	controllerConfig, err := config.LoadConfig(*configFilePath)
	if err != nil {
		klog.Errorf("failed to load the multus-daemon configuration: %v", err)
		os.Exit(ErrorLoadingConfig)
	}

	stopChannel := make(chan struct{})

	podNetworksController, err := newController(stopChannel, controllerConfig)
	if err != nil {
		klog.Errorf("failed to instantiate the %s controller: %v", controller.AdvertisedName, err)
		close(stopChannel) // deferred calls will not be called after os.Exit is called
		os.Exit(ErrorBuildingController)
	}

	defer close(stopChannel)
	handleSignals(stopChannel, os.Interrupt)
	podNetworksController.Start(stopChannel)
}

func newController(stopChannel chan struct{}, configuration *config.Multus) (*controller.PodNetworksController, error) {
	klog.V(logging.Debug).Infof("creating pod update controller ...")
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to implicitly generate the kubeconfig: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create the K8S client: %v", err)
	}

	nadClientSet, err := nadclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create the net-attach-def client: %v", err)
	}

	const noResyncPeriod = 0
	podInformerFactory := v1coreinformerfactory.NewSharedInformerFactoryWithOptions(
		k8sClient, noResyncPeriod, listenOnCoLocatedNode())

	nadInformerFactory := nadinformers.NewSharedInformerFactory(nadClientSet, noResyncPeriod)

	eventBroadcaster := newEventBroadcaster(k8sClient)

	const shortTimeout = 5 * time.Second
	containerRuntime, err := cri.NewRuntime(configuration.CriSocketPath, shortTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRI runtime (%s): %v", configuration.CriSocketPath, err)
	}

	podNetworksController, err := controller.NewPodNetworksController(
		podInformerFactory,
		nadInformerFactory,
		eventBroadcaster,
		newEventRecorder(eventBroadcaster),
		k8sClient,
		nadClientSet,
		containerRuntime,
		multuscni.NewClient(configuration.MultusSocketPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create the pod networks controller: %v", err)
	}

	klog.V(logging.Debug).Infof("starting informer factories ...")
	podInformerFactory.Start(stopChannel)
	nadInformerFactory.Start(stopChannel)

	klog.V(logging.Debug).Infof("finished creating the pod networks controller")
	return podNetworksController, nil
}

func listenOnCoLocatedNode() v1coreinformerfactory.SharedInformerOption {
	return v1coreinformerfactory.WithTweakListOptions(
		func(options *v1.ListOptions) {
			const (
				nodeNameEnvVariable = "NODE_NAME"
			)
			// The selector for the pods that this controller instance will watch/reconcile
			selectorSet := fields.Set{
				// select pods scheduled only on the node on which this controller instance is running
				"spec.nodeName": os.Getenv(nodeNameEnvVariable),
				// select pods with a phase Running to avoid interfering with the cni-plugin works
				// when pods got created/deleted
				// see https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase
				"status.phase": string(corev1.PodRunning),
			}
			options.FieldSelector = fields.SelectorFromSet(selectorSet).String()
		})
}

func newEventBroadcaster(k8sClientset kubernetes.Interface) record.EventBroadcaster {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sClientset.CoreV1().Events(v1.NamespaceAll)})
	return eventBroadcaster
}

func newEventRecorder(broadcaster record.EventBroadcaster) record.EventRecorder {
	return broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controller.AdvertisedName})
}

func handleSignals(stopChannel chan struct{}, signals ...os.Signal) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, signals...)
	go func() {
		<-signalChannel
		stopChannel <- struct{}{}
	}()
}

func controllerVersion() string {
	if commitSHA, wasFound := os.LookupEnv("DYNAMIC_NETWORK_CONTROLLER_COMMIT_HASH"); wasFound {
		return commitSHA
	}
	return "SHA-NOT-FOUND"
}
