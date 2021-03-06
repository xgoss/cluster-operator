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

package machineset

import (
	"fmt"
	"time"

	v1batch "k8s.io/api/batch/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	kubeclientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"

	"github.com/openshift/cluster-operator/pkg/ansible"
	"github.com/openshift/cluster-operator/pkg/kubernetes/pkg/util/metrics"

	clusteroperator "github.com/openshift/cluster-operator/pkg/apis/clusteroperator/v1alpha1"
	clusteroperatorclientset "github.com/openshift/cluster-operator/pkg/client/clientset_generated/clientset"
	informers "github.com/openshift/cluster-operator/pkg/client/informers_generated/externalversions/clusteroperator/v1alpha1"
	lister "github.com/openshift/cluster-operator/pkg/client/listers_generated/clusteroperator/v1alpha1"
	"github.com/openshift/cluster-operator/pkg/controller"
)

const (
	// maxRetries is the number of times a service will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the
	// sequence of delays between successive queuings of a service.
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15

	controllerLogName = "machineSet"

	// jobPrefix is used when generating a name for the configmap and job used for each
	// Ansible execution.
	jobPrefix = "provision-machineset-"

	masterProvisioningPlaybook = "playbooks/aws/openshift-cluster/provision.yml"

	computeProvisioningPlaybook = "playbooks/aws/openshift-cluster/provision_nodes.yml"
)

var (
	machineSetKind = clusteroperator.SchemeGroupVersion.WithKind("MachineSet")
	clusterKind    = clusteroperator.SchemeGroupVersion.WithKind("Cluster")
)

// NewMachineSetController returns a new *MachineSetController.
func NewMachineSetController(
	clusterInformer informers.ClusterInformer,
	machineSetInformer informers.MachineSetInformer,
	jobInformer batchinformers.JobInformer,
	kubeClient kubeclientset.Interface,
	clusteroperatorClient clusteroperatorclientset.Interface,
	ansibleImage string,
	ansibleImagePullPolicy kapi.PullPolicy,
) *MachineSetController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("clusteroperator_machine_set_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

	logger := log.WithField("controller", controllerLogName)

	c := &MachineSetController{
		client:     clusteroperatorClient,
		kubeClient: kubeClient,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineSet"),
		logger:     logger,
	}

	machineSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addMachineSet,
		UpdateFunc: c.updateMachineSet,
		DeleteFunc: c.deleteMachineSet,
	})
	c.machineSetsLister = machineSetInformer.Lister()
	c.machineSetsSynced = machineSetInformer.Informer().HasSynced

	clusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addCluster,
		UpdateFunc: c.updateCluster,
	})
	c.clustersLister = clusterInformer.Lister()
	c.clustersSynced = clusterInformer.Informer().HasSynced

	jobOwnerControl := &jobOwnerControl{controller: c}
	c.jobControl = controller.NewJobControl(jobPrefix, machineSetKind, kubeClient, jobInformer.Lister(), jobOwnerControl, logger)
	jobInformer.Informer().AddEventHandler(c.jobControl)
	c.jobsSynced = jobInformer.Informer().HasSynced

	c.syncHandler = c.syncMachineSet
	c.enqueueMachineSet = c.enqueue
	c.ansibleGenerator = ansible.NewJobGenerator(ansibleImage, ansibleImagePullPolicy)

	return c
}

// MachineSetController manages provisioning machine sets.
type MachineSetController struct {
	client     clusteroperatorclientset.Interface
	kubeClient kubeclientset.Interface

	// To allow injection of syncMachineSet for testing.
	syncHandler func(hKey string) error

	// To allow injection of mock ansible generator for testing
	ansibleGenerator ansible.JobGenerator

	jobControl controller.JobControl

	// used for unit testing
	enqueueMachineSet func(machineSet *clusteroperator.MachineSet)

	// machineSetsLister is able to list/get machine sets and is populated by the shared informer passed to
	// NewMachineSetController.
	machineSetsLister lister.MachineSetLister
	// machineSetsSynced returns true if the machine set shared informer has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	machineSetsSynced cache.InformerSynced

	// clustersLister is able to list/get clusters and is populated by the shared informer passed to
	// NewClusterController.
	clustersLister lister.ClusterLister
	// clustersSynced returns true if the cluster shared informer has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	clustersSynced cache.InformerSynced

	// jobsSynced returns true of the job shared informer has been synced at least once.
	jobsSynced cache.InformerSynced

	// MachineSets that need to be synced
	queue workqueue.RateLimitingInterface

	logger *log.Entry
}

func (c *MachineSetController) addMachineSet(obj interface{}) {
	ms := obj.(*clusteroperator.MachineSet)
	loggerForMachineSet(c.logger, ms).Debugf("enqueueing added machine set")
	c.enqueueMachineSet(ms)
}

func (c *MachineSetController) updateMachineSet(old, cur interface{}) {
	ms := cur.(*clusteroperator.MachineSet)
	loggerForMachineSet(c.logger, ms).Debugf("enqueueing updated machine set")
	c.enqueueMachineSet(ms)
}

func (c *MachineSetController) deleteMachineSet(obj interface{}) {
	ms, ok := obj.(*clusteroperator.MachineSet)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		ms, ok = tombstone.Obj.(*clusteroperator.MachineSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a MachineSet %#v", obj))
			return
		}
	}
	loggerForMachineSet(c.logger, ms).Debugf("enqueueing deleted machine set")
	c.enqueueMachineSet(ms)
}

func (c *MachineSetController) addCluster(obj interface{}) {
	cluster := obj.(*clusteroperator.Cluster)
	logger := loggerForCluster(c.logger, cluster)
	machineSets, err := c.machineSetsForCluster(cluster)
	if err != nil {
		logger.Errorf("Cannot retrieve machine sets for cluster: %v", err)
		utilruntime.HandleError(err)
		return
	}

	for _, machineSet := range machineSets {
		loggerForMachineSet(logger, machineSet).Debugf("enqueueing machine set for created cluster")
		c.enqueueMachineSet(machineSet)
	}
}

func (c *MachineSetController) updateCluster(old, obj interface{}) {
	cluster := obj.(*clusteroperator.Cluster)
	logger := loggerForCluster(c.logger, cluster)
	machineSets, err := c.machineSetsForCluster(cluster)
	if err != nil {
		logger.Errorf("Cannot retrieve machine sets for cluster: %v", err)
		utilruntime.HandleError(err)
		return
	}
	for _, machineSet := range machineSets {
		loggerForMachineSet(logger, machineSet).Debugf("enqueueing machine set for update cluster")
		c.enqueueMachineSet(machineSet)
	}
}

func (c *MachineSetController) machineSetsForCluster(cluster *clusteroperator.Cluster) ([]*clusteroperator.MachineSet, error) {
	clusterMachineSets := []*clusteroperator.MachineSet{}
	allMachineSets, err := c.machineSetsLister.MachineSets(cluster.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, machineSet := range allMachineSets {
		if metav1.IsControlledBy(machineSet, cluster) {
			clusterMachineSets = append(clusterMachineSets, machineSet)
		}
	}
	return clusterMachineSets, nil
}

// Runs c; will not return until stopCh is closed. workers determines how many
// machine sets will be handled in parallel.
func (c *MachineSetController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Infof("Starting machine set controller")
	defer c.logger.Infof("Shutting down machine set controller")

	if !controller.WaitForCacheSync("machineset", stopCh, c.machineSetsSynced, c.clustersSynced, c.jobsSynced) {
		c.logger.Errorf("Could not sync caches for machineset controller")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MachineSetController) enqueue(machineSet *clusteroperator.MachineSet) {
	key, err := controller.KeyFunc(machineSet)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", machineSet, err))
		return
	}

	c.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *MachineSetController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MachineSetController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *MachineSetController) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	logger := c.logger.WithField("machineset", key)

	logger.Errorf("error syncing machine set: %v", err)
	if c.queue.NumRequeues(key) < maxRetries {
		logger.Infof("retrying machine set")
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	logger.Infof("dropping machine set out of the queue: %v", err)
	c.queue.Forget(key)
}

// syncMachineSet will sync the machine set with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (c *MachineSetController) syncMachineSet(key string) error {
	startTime := time.Now()
	logger := c.logger.WithField("machineset", key)
	logger.Debugln("Started syncing machine set")
	defer logger.WithField("duration", time.Since(startTime)).Debugln("Finished syncing machine set")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("cannot parse machine set key %q: %v", key, err))
		return nil
	}
	if len(ns) == 0 || len(name) == 0 {
		utilruntime.HandleError(fmt.Errorf("invalid machineset key %q: either namespace or name is missing", key))
		return nil
	}

	machineSet, err := c.machineSetsLister.MachineSets(ns).Get(name)
	if errors.IsNotFound(err) {
		logger.Debugln("machine set has been deleted")
		c.jobControl.ObserveOwnerDeletion(key)
		return nil
	}
	if err != nil {
		return err
	}

	shouldProvisionMachineSet, err := c.shouldProvision(machineSet)
	if err != nil {
		return err
	}

	jobFactory, err := c.getJobFactory(machineSet)
	if err != nil {
		return err
	}

	job, isJobNew, err := c.jobControl.ControlJobs(key, machineSet, shouldProvisionMachineSet, jobFactory)
	if err != nil {
		return err
	}

	if !shouldProvisionMachineSet {
		return nil
	}

	switch {
	// New job has not been created, so an old job must exist. Set the machine
	// set to not provisioning as the old job is deleted.
	case job == nil:
		return c.setMachineSetToNotProvisioning(machineSet)
	// Job was not newly created, so sync machine set status with job.
	case !isJobNew:
		logger.Debugln("provisioning job exists, will sync with job")
		return c.syncMachineSetStatusWithJob(machineSet, job)
	// MachineSet should have a job to provision the current spec but it was not
	// found.
	case isMachineSetProvisioning(machineSet):
		return c.setJobNotFoundStatus(machineSet)
	// New job created for new provisioning
	default:
		return nil
	}
}

func (c *MachineSetController) clusterForMachineSet(machineSet *clusteroperator.MachineSet) (*clusteroperator.Cluster, error) {
	controllerRef := metav1.GetControllerOf(machineSet)
	if controllerRef.Kind != clusterKind.Kind {
		return nil, nil
	}
	cluster, err := c.clustersLister.Clusters(machineSet.Namespace).Get(controllerRef.Name)
	if err != nil {
		return nil, err
	}
	if cluster.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil, nil
	}
	return cluster, nil
}

func (c *MachineSetController) shouldProvision(machineSet *clusteroperator.MachineSet) (bool, error) {
	if machineSet.Status.ProvisionedJobGeneration == machineSet.Generation {
		return false, nil
	}
	cluster, err := c.clusterForMachineSet(machineSet)
	if err != nil {
		return false, err
	}
	if !cluster.Status.Provisioned || cluster.Status.ProvisionedJobGeneration != cluster.Generation {
		return false, nil
	}
	switch machineSet.Spec.NodeType {
	case clusteroperator.NodeTypeMaster:
		return true, nil
	case clusteroperator.NodeTypeCompute:
		masterMachineSet, err := c.machineSetsLister.MachineSets(machineSet.Namespace).Get(cluster.Status.MasterMachineSetName)
		if err != nil {
			return false, err
		}
		// Only provision compute nodes if openshift has been installed on
		// master nodes.
		// We need to verify that the generation of the master machine set
		// has not been changed since the installation. If the generation
		// has changed, then the installation is no longer valid. The master
		// controller needs to re-work the installation first.
		masterInstalled := masterMachineSet.Status.Installed &&
			masterMachineSet.Status.InstalledJobGeneration == masterMachineSet.Generation
		return masterInstalled, nil
	default:
		return false, fmt.Errorf("unknown node type")
	}
}

type jobOwnerControl struct {
	controller *MachineSetController
}

func (c *jobOwnerControl) GetOwnerKey(owner metav1.Object) (string, error) {
	return controller.KeyFunc(owner)
}

func (c *jobOwnerControl) GetOwner(namespace string, name string) (metav1.Object, error) {
	return c.controller.machineSetsLister.MachineSets(namespace).Get(name)
}

func (c *jobOwnerControl) OnOwnedJobEvent(owner metav1.Object) {
	machineSet, ok := owner.(*clusteroperator.MachineSet)
	if !ok {
		c.controller.logger.WithFields(log.Fields{"owner": owner.GetName(), "namespace": owner.GetNamespace()}).
			Errorf("attempt to enqueue owner that is not a machineset")
		return
	}
	c.controller.enqueueMachineSet(machineSet)
}

type jobFactory func(string) (*v1batch.Job, *kapi.ConfigMap, error)

func (f jobFactory) BuildJob(name string) (*v1batch.Job, *kapi.ConfigMap, error) {
	return f(name)
}

func (c *MachineSetController) getJobFactory(machineSet *clusteroperator.MachineSet) (controller.JobFactory, error) {
	cluster, err := c.clusterForMachineSet(machineSet)
	if err != nil {
		return nil, err
	}
	return jobFactory(func(name string) (*v1batch.Job, *kapi.ConfigMap, error) {
		vars, err := ansible.GenerateMachineSetVars(cluster, machineSet)
		if err != nil {
			return nil, nil, err
		}
		var playbook string
		if machineSet.Spec.NodeType == clusteroperator.NodeTypeMaster {
			playbook = masterProvisioningPlaybook
		} else {
			playbook = computeProvisioningPlaybook
		}
		job, configMap := c.ansibleGenerator.GeneratePlaybookJob(name, &cluster.Spec.Hardware, playbook, ansible.DefaultInventory, vars)
		return job, configMap, nil
	}), nil
}

// setMachineSetToNotProvisioning updates the HardwareProvisioning condition
// for the machine set to reflect that a machine set that had an in-progress
// provision is no longer provisioning due to a change in the spec of the
// machine set.
func (c *MachineSetController) setMachineSetToNotProvisioning(original *clusteroperator.MachineSet) error {
	machineSet := original.DeepCopy()

	controller.SetMachineSetCondition(
		machineSet,
		clusteroperator.MachineSetHardwareProvisioning,
		kapi.ConditionFalse,
		controller.ReasonSpecChanged,
		"Spec changed. New provisioning needed",
	)

	return c.updateMachineSetStatus(original, machineSet)
}

// syncMachineSetStatusWithJob update the status of the machine set to
// reflect the current status of the job that is provisioning the machine set.
// If the job completed successfully, the machine set will be marked as
// provisioned.
// If the job completed with a failure, the machine set will be marked as
// not provisioned.
// If the job is still in progress, the machine set will be marked as
// provisioning.
func (c *MachineSetController) syncMachineSetStatusWithJob(original *clusteroperator.MachineSet, job *v1batch.Job) error {
	machineSet := original.DeepCopy()

	jobCompleted := jobCondition(job, v1batch.JobComplete)
	jobFailed := jobCondition(job, v1batch.JobFailed)
	switch {
	// Provision job completed successfully
	case jobCompleted != nil && jobCompleted.Status == kapi.ConditionTrue:
		reason := controller.ReasonJobCompleted
		message := fmt.Sprintf("Job %s/%s completed at %v", job.Namespace, job.Name, jobCompleted.LastTransitionTime)
		controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioning, kapi.ConditionFalse, reason, message)
		controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioned, kapi.ConditionTrue, reason, message)
		controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioningFailed, kapi.ConditionFalse, reason, message)
		machineSet.Status.Provisioned = true
		machineSet.Status.ProvisionedJobGeneration = machineSet.Generation
	// Provision job failed
	case jobFailed != nil && jobFailed.Status == kapi.ConditionTrue:
		reason := controller.ReasonJobFailed
		message := fmt.Sprintf("Job %s/%s failed at %v, reason: %s", job.Namespace, job.Name, jobFailed.LastTransitionTime, jobFailed.Reason)
		controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioning, kapi.ConditionFalse, reason, message)
		controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioningFailed, kapi.ConditionTrue, reason, message)
		// ProvisionedJobGeneration is set even when the job failed because we
		// do not want to run the provision job again until there have been
		// changes in the spec of the machine set.
		machineSet.Status.ProvisionedJobGeneration = machineSet.Generation
	default:
		reason := controller.ReasonJobRunning
		message := fmt.Sprintf("Job %s/%s is running since %v. Pod completions: %d, failures: %d", job.Namespace, job.Name, job.Status.StartTime, job.Status.Succeeded, job.Status.Failed)
		controller.SetMachineSetCondition(
			machineSet,
			clusteroperator.MachineSetHardwareProvisioning,
			kapi.ConditionTrue,
			reason,
			message,
			func(old, new clusteroperator.MachineSetCondition) bool {
				return new.Message != old.Message
			},
		)
	}

	return c.updateMachineSetStatus(original, machineSet)

}

func (c *MachineSetController) setJobNotFoundStatus(original *clusteroperator.MachineSet) error {
	machineSet := original.DeepCopy()
	reason := controller.ReasonJobMissing
	message := "Provisioning job not found."
	controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioning, kapi.ConditionFalse, reason, message)
	controller.SetMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioningFailed, kapi.ConditionTrue, reason, message)
	return c.updateMachineSetStatus(original, machineSet)
}

func (c *MachineSetController) updateMachineSetStatus(original, machineSet *clusteroperator.MachineSet) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return controller.PatchMachineSetStatus(c.client, original, machineSet)
	})
}

func isMachineSetProvisioning(machineSet *clusteroperator.MachineSet) bool {
	provisioning := controller.FindMachineSetCondition(machineSet, clusteroperator.MachineSetHardwareProvisioning)
	return provisioning != nil && provisioning.Status == kapi.ConditionTrue
}

func jobCondition(job *v1batch.Job, conditionType v1batch.JobConditionType) *v1batch.JobCondition {
	for i, condition := range job.Status.Conditions {
		if condition.Type == conditionType {
			return &job.Status.Conditions[i]
		}
	}
	return nil
}

func loggerForMachineSet(logger log.FieldLogger, machineSet *clusteroperator.MachineSet) log.FieldLogger {
	return logger.WithFields(log.Fields{"machineset": machineSet.Name, "namespace": machineSet.Namespace})
}

func loggerForCluster(logger log.FieldLogger, cluster *clusteroperator.Cluster) log.FieldLogger {
	return logger.WithFields(log.Fields{"cluster": cluster.Name, "namespace": cluster.Namespace})
}
