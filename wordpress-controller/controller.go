/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	// _ "k8s.io/client-go/pkg/api/install"
	// _ "k8s.io/client-go/pkg/apis/extensions/install"

	apiv1 "k8s.io/api/core/v1"
	apiutil "k8s.io/apimachinery/pkg/util/intstr"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	wpv1 "github.com/k8s-controllers/wordpress-controller/pkg/apis/wordpresscontroller/v1"
	clientset "github.com/k8s-controllers/wordpress-controller/pkg/generated/clientset/versioned"
	websitescheme "github.com/k8s-controllers/wordpress-controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/k8s-controllers/wordpress-controller/pkg/generated/informers/externalversions/wordpresscontroller/v1"
	listers "github.com/k8s-controllers/wordpress-controller/pkg/generated/listers/wordpresscontroller/v1"
)

const controllerAgentName = "wordpress-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Website is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Website fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Website"
	// MessageResourceSynced is the message used for an Event fired when a Website
	// is synced successfully
	MessageResourceSynced = "Website synced successfully"
)

const (
	PASSWORD    = "mysecretpassword"
	MINIKUBE_IP = "192.168.64.12"
)

// Controller is the controller implementation for Website resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// websiteclientset is a clientset for our own API group
	websiteclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	websitesLister    listers.WebsiteLister
	websitesSynced    cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	websiteclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	websiteInformer informers.WebsiteInformer) *Controller {

	// Create event broadcaster
	// Add website-controller types to the default Kubernetes Scheme so Events can be
	// logged for website-controller types.
	utilruntime.Must(websitescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		websiteclientset:  websiteclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		websitesLister:    websiteInformer.Lister(),
		websitesSynced:    websiteInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Website"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Website resources change
	websiteInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueWebsite,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueWebsite(new)
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Website resource will enqueue that Website resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Website controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.websitesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Website resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Website resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Website resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	fmt.Println("Inside syncHandler 1")
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Website resource with this namespace/name
	website, err := c.websitesLister.Websites(namespace).Get(name)
	if err != nil {
		// The Website resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("website '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	fmt.Println("Inside syncHandler 2")
	deploymentName := website.Spec.DeploymentName
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	var verifyCmd string
	var actionHistory []string
	var serviceIP string
	var servicePort string
	var setupCommands []string

	// Get the deployment with the name specified in Website.spec
	_, err = c.deploymentsLister.Deployments(website.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		fmt.Printf("Received request to create CRD %s\n", deploymentName)
		serviceIP, servicePort, setupCommands, verifyCmd = createDeployment(website, c)
		// deployment, err = c.kubeclientset.AppsV1().Deployments(website.Namespace).Create(newDeployment(website))
		for _, cmds := range setupCommands {
			actionHistory = append(actionHistory, cmds)
		}
		fmt.Printf("Setup Commands: %v\n", setupCommands)
		fmt.Printf("Verify using: %v\n", verifyCmd)
		// Finally, we update the status block of the Website resource to reflect the
		// current state of the world
		err = c.updateWebsiteStatus(website, &actionHistory, verifyCmd, serviceIP, servicePort, "READY")
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("CRD %s created\n", deploymentName)
		fmt.Printf("Check using: kubectl describe website %s \n", deploymentName)
	}

	wp, err := c.websiteclientset.DynamicV1().Websites(website.Namespace).Get(deploymentName, metav1.GetOptions{})

	actionHistory = wp.Status.ActionHistory
	serviceIP = wp.Status.ServiceIP
	servicePort = wp.Status.ServicePort
	verifyCmd = wp.Status.VerifyCmd
	fmt.Printf("Action History:[%s]\n", actionHistory)
	fmt.Printf("Service IP:[%s]\n", serviceIP)
	fmt.Printf("Service Port:[%s]\n", servicePort)
	fmt.Printf("Verify cmd: %v\n", verifyCmd)

	setupCommands = canonicalize(website.Spec.Commands)

	var commandsToRun []string
	commandsToRun = getCommandsToRun(actionHistory, setupCommands)
	fmt.Printf("commandsToRun: %v\n", commandsToRun)

	if len(commandsToRun) > 0 {
		err = c.updateWebsiteStatus(website, &actionHistory, verifyCmd, serviceIP, servicePort, "UPDATING")
		if err != nil {
			return err
		}
		updateCRD(wp, c, commandsToRun)
		for _, cmds := range commandsToRun {
			actionHistory = append(actionHistory, cmds)
		}
		err = c.updateWebsiteStatus(website, &actionHistory, verifyCmd, serviceIP, servicePort, "READY")
		if err != nil {
			return err
		}
	}

	c.recorder.Event(website, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil

}

func getCommandsToRun(actionHistory []string, setupCommands []string) []string {
	var commandsToRun []string
	for _, v := range setupCommands {
		var found = false
		for _, v1 := range actionHistory {
			if v == v1 {
				found = true
			}
		}
		if !found {
			commandsToRun = append(commandsToRun, v)
		}
	}
	fmt.Printf("-- commandsToRun: %v--\n", commandsToRun)
	return commandsToRun
}

func canonicalize(setupCommands1 []string) []string {
	var setupCommands []string
	//Convert setupCommands to Lower case
	for _, cmd := range setupCommands1 {
		setupCommands = append(setupCommands, strings.ToLower(cmd))
	}
	return setupCommands
}

func (c *Controller) updateWebsiteStatus(website *wpv1.Website, actionHistory *[]string, verifyCmd string, serviceIP string, servicePort string,
	status string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	websiteCopy := website.DeepCopy()
	websiteCopy.Status.AvailableReplicas = 1

	//websiteCopy.Status.ActionHistory = strings.Join(*actionHistory, " ")
	websiteCopy.Status.VerifyCmd = verifyCmd
	websiteCopy.Status.ActionHistory = *actionHistory
	websiteCopy.Status.ServiceIP = serviceIP
	websiteCopy.Status.ServicePort = servicePort
	websiteCopy.Status.Status = status

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Website resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.websiteclientset.DynamicV1().Websites(website.Namespace).Update(websiteCopy)
	return err
}

// enqueueWebsite takes a Website resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Website.
func (c *Controller) enqueueWebsite(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Website resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Website resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Website, we should not do anything more
		// with it.
		if ownerRef.Kind != "Website" {
			return
		}

		website, err := c.websitesLister.Websites(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of website '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueWebsite(website)
		return
	}
}

func updateCRD(website *wpv1.Website, c *Controller, setupCommands []string) {
	serviceIP := website.Status.ServiceIP
	servicePort := website.Status.ServicePort

	//setupCommands1 := website.Spec.Commands
	//var setupCommands []string

	//Convert setupCommands to Lower case
	//for _, cmd := range setupCommands1 {
	//	 setupCommands = append(setupCommands, strings.ToLower(cmd))
	//}

	fmt.Printf("Service IP:[%s]\n", serviceIP)
	fmt.Printf("Service Port:[%s]\n", servicePort)
	fmt.Printf("Command:[%s]\n", setupCommands)

	if len(setupCommands) > 0 {
		// file := createTempDBFile(setupCommands)
		fmt.Println("Now setting up the database")
		// setupDatabase(serviceIP, servicePort, file)
	}
}

func createDeployment(website *wpv1.Website, c *Controller) (string, string, []string, string) {

	deploymentsClient := c.kubeclientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	// image := website.Spec.Image
	// 	var env []corev1.EnvVar
	// envFrom := website.Spec.EnvFrom
	// username := env["WORDPRESS_DB_USER"]
	// password := env.WORDPRESS_DB_PASSWORD
	// database := env.WORDPRESS_DB_NAME
	// host := env.WORDPRESS_DB_HOST
	// wpPrefix := env.WORDPRESS_TABLE_PREFIX
	// username := website.Spec.Username
	// password := website.Spec.Password
	// database := website.Spec.Database
	deploymentName := website.Spec.DeploymentName
	setupCommands := canonicalize(website.Spec.Commands)

	// fmt.Printf("   Deployment:%v, Image:%v, User:%v\n", deploymentName, image, username)
	// fmt.Printf("   Password:%v, Database:%v\n", password, database)
	// fmt.Printf("SetupCmds:%v\n", setupCommands)
	// env := []v1.EnvVar{
	// 	{
	// 		Name:  "WORDPRESS_DB_HOST",
	// 		Value: "websites.cujxcm1nmul2.us-east-1.rds.amazonaws.com",
	// 	},
	// 	{
	// 		Name:  "WORDPRESS_DB_USER",
	// 		Value: "root",
	// 	},
	// 	{
	// 		Name:  "WORDPRESS_DB_PASSWORD",
	// 		Value: "client01",
	// 	},
	// 	{
	// 		Name:  "WORDPRESS_DB_NAME",
	// 		Value: "wordpress",
	// 	},
	// 	{
	// 		Name:  "WORDPRESS_TABLE_PREFIX",
	// 		Value: "WP_",
	// 	},
	// }
	deploymentManifest := `
	apiVersion: apiextensions.k8s.io/v1beta1
	kind: CustomResourceDefinition
	metadata:
		name: websites.dynamic.microowl.com
	spec:
		group: dynamic.microowl.com
		version: v1
		names:
			kind: Website
			plural: websites
		scope: Namespaced
	`
	deployment := &appsv1.Deployment{}
	dec := k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(deploymentManifest)), 1000)

	if err := dec.Decode(&deployment); err != nil {
		return "serviceIP", "servicePort", setupCommands, "verifyCmdString"
	}
	fmt.Println("%+v", deployment)

	// // deploymen := website.wp.WebPodTemplateSpec()
	// deployment := &appsv1.Deployment{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: deploymentName,
	// 	},
	// 	Spec: appsv1.DeploymentSpec{
	// 		Replicas: int32Ptr(1),
	// 		Selector: &metav1.LabelSelector{
	// 			MatchLabels: map[string]string{
	// 				"app": deploymentName,
	// 			},
	// 		},
	// 		Template: apiv1.PodTemplateSpec{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Labels: map[string]string{
	// 					"app": deploymentName,
	// 				},
	// 			},

	// 			Spec: apiv1.PodSpec{
	// 				Containers: []apiv1.Container{
	// 					{
	// 						Name:  deploymentName,
	// 						Image: image,
	// 						Ports: []apiv1.ContainerPort{
	// 							{
	// 								ContainerPort: 80,
	// 							},
	// 						},
	// 						ReadinessProbe: &apiv1.Probe{
	// 							Handler: apiv1.Handler{
	// 								TCPSocket: &apiv1.TCPSocketAction{
	// 									Port: apiutil.FromInt(8080),
	// 								},
	// 							},
	// 							InitialDelaySeconds: 5,
	// 							TimeoutSeconds:      60,
	// 							PeriodSeconds:       2,
	// 						},
	// 						Env: env,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	fmt.Printf("------------------------------\n")

	// Create Service
	fmt.Printf("Creating service...\n")
	serviceClient := c.kubeclientset.CoreV1().Services(apiv1.NamespaceDefault)
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "my-port",
					Port:       80,
					TargetPort: apiutil.FromInt(8080),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			Type: apiv1.ServiceTypeNodePort,
		},
	}

	result1, err1 := serviceClient.Create(service)
	if err1 != nil {
		panic(err1)
	}
	fmt.Printf("Created service %q.\n", result1.GetObjectMeta().GetName())
	fmt.Printf("------------------------------\n")

	// Parse ServiceIP and Port
	// Minikube VM IP
	serviceIP := MINIKUBE_IP

	nodePort1 := result1.Spec.Ports[0].NodePort
	nodePort := fmt.Sprint(nodePort1)
	servicePort := nodePort
	//fmt.Printf("NodePort:[%v]", nodePort)

	//fmt.Println("About to get Pods")
	time.Sleep(time.Second * 5)

	for {
		readyPods := 0
		pods := getPods(c, deploymentName)
		//fmt.Println("Got Pods:: %s", pods)
		for _, d := range pods.Items {
			//fmt.Printf(" * %s %s \n", d.Name, d.Status)
			podConditions := d.Status.Conditions
			for _, podCond := range podConditions {
				if podCond.Type == corev1.PodReady {
					if podCond.Status == corev1.ConditionTrue {
						//fmt.Println("Pod is running.")
						readyPods++
						//fmt.Printf("ReadyPods:%d\n", readyPods)
						//fmt.Printf("TotalPods:%d\n", len(pods.Items))
					}
				}
			}
		}
		if readyPods >= len(pods.Items) {
			break
		} else {
			fmt.Println("Waiting for Pod to get ready.")
			// Sleep for the Pod to become active
			time.Sleep(time.Second * 4)
		}
	}

	// Wait couple of seconds more just to give the Pod some more time.
	time.Sleep(time.Second * 2)

	if len(setupCommands) > 0 {
		// file := createTempDBFile(setupCommands)
		fmt.Println("Now setting up the database")
		// setupDatabase(serviceIP, servicePort, file)
	}

	// List Deployments
	//fmt.Printf("Listing deployments in namespace %q:\n", apiv1.NamespaceDefault)
	//list, err := deploymentsClient.List(metav1.ListOptions{})
	//if err != nil {
	//        panic(err)
	//}
	//for _, d := range list.Items {
	//        fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	//}

	verifyCmd := strings.Fields("psql -h " + serviceIP + " -p " + nodePort + " -U <user> " + " -d <db-name>")
	var verifyCmdString = strings.Join(verifyCmd, " ")
	fmt.Printf("VerifyCmd: %v\n", verifyCmd)
	return serviceIP, servicePort, setupCommands, verifyCmdString
}

func getPods(c *Controller, deploymentName string) *apiv1.PodList {
	// TODO(devkulkarni): This is returning all Pods. We should change this
	// to only return Pods whose Label matches the Deployment Name.
	pods, err := c.kubeclientset.CoreV1().Pods("default").List(metav1.ListOptions{
		LabelSelector: deploymentName,
		// LabelSelector: metav1.LabelSelector{
		// 	MatchLabels: map[string]string{
		// 		"app": deploymentName,
		// 	},
		// },
	})
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
	if err != nil {
		fmt.Printf("%s", err)
	}
	fmt.Println("Got Pods: %s", pods)
	return pods
}

// func createTempDBFile(setupCommands []string) *os.File {
// 	file, err := ioutil.TempFile("/tmp", "create-db1")
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Printf("Database setup file:%s\n", file.Name())

// 	for _, command := range setupCommands {
// 		//fmt.Printf("Command: %v\n", command)
// 		// TODO: Interpolation of variables
// 		file.WriteString(command)
// 		file.WriteString("\n")
// 	}
// 	file.Sync()
// 	file.Close()
// 	return file
// }

// newDeployment creates a new Deployment for a Website resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Website resource that 'owns' it.
func newDeployment(website *wpv1.Website) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "wordpress",
		"controller": website.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Spec.DeploymentName,
			Namespace: website.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(website, wpv1.SchemeGroupVersion.WithKind("Website")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: website.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "wordpress",
							Image: "aajipsum/wordpress:latest",
						},
					},
				},
			},
		},
	}
}
func int32Ptr(i int32) *int32 { return &i }
