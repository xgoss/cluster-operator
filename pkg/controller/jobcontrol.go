/*
Copyright 2018 The Kubernetes Authors.

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

package controller

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	kbatch "k8s.io/api/batch/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	kubeclientset "k8s.io/client-go/kubernetes"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"

	clusteroperator "github.com/openshift/cluster-operator/pkg/apis/clusteroperator/v1alpha1"
)

// JobControl is used to control jobs that are needed by a controller.
type JobControl interface {
	// OnAdd from ResourceEventHandler for a Job informer
	OnAdd(obj interface{})
	// OnUpdate from ResourceEventHandler for a Job informer
	OnUpdate(oldObj, newObj interface{})
	// OnDelete from ResourceEventHandler for a Job informer
	OnDelete(obj interface{})

	// ControlJobs handles running/deleting jobs for the owner.
	//
	// ownerKey: key to identify the owner (namespace/name)
	// owner: owner owning the jobs
	// buildNewJob: true if a new job should be built if one does not already
	//   exist
	// jobFactory: function to call to build a new job
	//
	// Return:
	//  (*kbatch.Job) current job
	//  (bool) true if the current job was newly created
	//  (error) error that prevented handling the jobs
	ControlJobs(
		ownerKey string,
		owner metav1.Object,
		buildNewJob bool,
		jobFactory JobFactory,
	) (*kbatch.Job, bool, error)

	// ObserveOwnerDeletion observes that the owner with the specified key was
	// deleted.
	ObserveOwnerDeletion(ownerKey string)
}

var _ cache.ResourceEventHandler = (JobControl)(nil)

// JobOwnerControl is a control supplied to a job control used to access
// and signal about owners of jobs controlled by the job control.
type JobOwnerControl interface {
	// GetOwnerKey gets the key that identifies the owner.
	GetOwnerKey(owner metav1.Object) (string, error)

	// GetOwner gets the owner that has the specified namespace and name.
	GetOwner(namespace string, name string) (metav1.Object, error)

	// OnOwnedJobEvent signals that a job owned by the specified owner has
	// been added, updated, or deleted.
	OnOwnedJobEvent(owner metav1.Object)
}

// JobFactory is used to build a job to be controlled by a JobControl.
type JobFactory interface {
	// BuildJob builds a job (and associated configmap) with the specified
	// name.
	// Note that BuildJob should not create the job (or the configmap) in the
	// API Server.
	BuildJob(name string) (*kbatch.Job, *kapi.ConfigMap, error)
}

type jobControl struct {
	jobPrefix    string
	ownerKind    schema.GroupVersionKind
	kubeClient   kubeclientset.Interface
	jobsLister   batchlisters.JobLister
	ownerControl JobOwnerControl
	logger       log.FieldLogger
	// A TTLCache of job creations/deletions we're expecting to see
	expectations *UIDTrackingControllerExpectations
}

// NewJobControl creates a new JobControl.
func NewJobControl(
	jobPrefix string,
	ownerKind schema.GroupVersionKind,
	kubeClient kubeclientset.Interface,
	jobsLister batchlisters.JobLister,
	ownerControl JobOwnerControl,
	logger log.FieldLogger,
) JobControl {

	return &jobControl{
		jobPrefix:    jobPrefix,
		ownerKind:    ownerKind,
		kubeClient:   kubeClient,
		jobsLister:   jobsLister,
		ownerControl: ownerControl,
		logger:       logger,
		expectations: NewUIDTrackingControllerExpectations(NewControllerExpectations()),
	}
}

func (c *jobControl) ControlJobs(ownerKey string, owner metav1.Object, buildNewJob bool, jobFactory JobFactory) (*kbatch.Job, bool, error) {
	logger := loggerForOwner(c.logger, owner)

	if !c.expectations.SatisfiedExpectations(ownerKey) {
		// expectations have not been met, come back later
		logger.Debugln("expectations have not been satisfied yet")
		return nil, false, nil
	}

	// Look through jobs that belong to the owner and that match the type
	// of job created by this control.
	// If the job's generation corresponds to the owner's current
	// generation, sync the owner status with the job's latest status.
	// If the job does not correspond to the owner's current generation,
	// delete the job.
	oldJobs := []*kbatch.Job{}
	var currentJob *kbatch.Job
	jobs, err := c.jobsLister.Jobs(owner.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, false, err
	}
	for _, job := range jobs {
		if !metav1.IsControlledBy(job, owner) {
			continue
		}
		if !c.isControlledJob(job) {
			continue
		}
		if jobOwnerGeneration(job) == owner.GetGeneration() {
			currentJob = job
		} else {
			loggerForJob(logger, job).
				Debugln("found job that does not correspond to owner's current generation")
			oldJobs = append(oldJobs, job)
		}
	}

	if len(oldJobs) > 0 {
		return nil, false, c.deleteOldJobs(ownerKey, oldJobs, logger)
	}

	if currentJob != nil {
		return currentJob, false, nil
	}

	if buildNewJob {
		job, err := c.createJob(ownerKey, owner, jobFactory, logger)
		return job, job != nil, err
	}

	return nil, false, nil
}

func (c *jobControl) ObserveOwnerDeletion(ownerKey string) {
	c.expectations.DeleteExpectations(ownerKey)
}

func (c *jobControl) isControlledJob(job *kbatch.Job) bool {
	return strings.HasPrefix(job.Name, c.jobPrefix)
}

func (c *jobControl) ownerForJob(job *kbatch.Job) (metav1.Object, error) {
	controllerRef := metav1.GetControllerOf(job)
	if controllerRef == nil {
		return nil, nil
	}
	if controllerRef.Kind != c.ownerKind.Kind {
		return nil, nil
	}
	owner, err := c.ownerControl.GetOwner(job.Namespace, controllerRef.Name)
	if err != nil || owner == nil {
		return owner, err
	}
	if owner.GetUID() != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil, nil
	}
	return owner, nil
}

func (c *jobControl) convertEventHandlerObjectToControlledJob(obj interface{}, lookAtTombstone bool) (*kbatch.Job, bool) {
	job, ok := obj.(*kbatch.Job)
	if !ok {
		if !lookAtTombstone {
			handleError(fmt.Errorf("object provided by job informer is not a Job %#v", obj), c.logger)
			return nil, false
		}
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			handleError(fmt.Errorf("couldn't get object from tombstone %#v", obj), c.logger)
			return nil, false
		}
		job, ok = tombstone.Obj.(*kbatch.Job)
		if !ok {
			handleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj), c.logger)
			return nil, false
		}
	}

	return job, c.isControlledJob(job)
}

func (c *jobControl) OnAdd(obj interface{}) {
	c.onJobEvent(obj, jobAdd)
}

func (c *jobControl) OnUpdate(old, obj interface{}) {
	c.onJobEvent(obj, jobUpdate)
}

func (c *jobControl) OnDelete(obj interface{}) {
	c.onJobEvent(obj, jobDelete)
}

const (
	jobAdd    = "Add"
	jobUpdate = "Update"
	jobDelete = "Delete"
)

func (c *jobControl) onJobEvent(obj interface{}, eventType string) {
	job, ok := obj.(*kbatch.Job)
	if !ok {
		if eventType != jobDelete {
			handleError(fmt.Errorf("object provided by job informer is not a Job %#v", obj), c.logger)
			return
		}
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			handleError(fmt.Errorf("couldn't get object from tombstone %#v", obj), c.logger)
			return
		}
		job, ok = tombstone.Obj.(*kbatch.Job)
		if !ok {
			handleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj), c.logger)
			return
		}
	}

	if !c.isControlledJob(job) {
		return
	}

	logger := loggerForJob(c.logger, job)

	owner, err := c.ownerForJob(job)
	if err != nil || owner == nil {
		logger.Warn("owner no longer exists for job")
		return
	}

	ownerKey, err := c.ownerControl.GetOwnerKey(owner)
	if err != nil {
		handleError(err, logger)
		return
	}

	switch eventType {
	case jobAdd:
		logger.Debug("job creation observed")
		c.expectations.CreationObserved(ownerKey)
		logger.Debugf("enqueueing owner for created job")
	case jobUpdate:
		logger.Debugf("enqueueing owner for updated job")
	case jobDelete:
		logger.Debug("job deletion observed")
		c.expectations.DeletionObserved(ownerKey, jobKey(job))
		logger.Debugf("enqueueing owner for deleted job")
	default:
		handleError(fmt.Errorf("unknown job event type"), logger)
		return
	}

	c.ownerControl.OnOwnedJobEvent(owner)
}

func (c *jobControl) createJob(ownerKey string, owner metav1.Object, jobFactory JobFactory, logger log.FieldLogger) (*kbatch.Job, error) {
	logger.Infof("Creating new %q job", c.jobPrefix)

	name := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s%s-", c.jobPrefix, owner.GetName()))

	if jobFactory == nil {
		logger.Warn("asked to build new job but no job factory supplied")
		return nil, nil
	}

	job, configMap, err := jobFactory.BuildJob(name)
	if err != nil {
		return nil, err
	}

	if job == nil {
		return nil, nil
	}

	ownerRef := metav1.NewControllerRef(owner, c.ownerKind)

	job.OwnerReferences = append(job.OwnerReferences, *ownerRef)
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations[clusteroperator.OwnerGenerationAnnotation] = fmt.Sprintf("%d", owner.GetGeneration())

	cleanUpConfigMap := true
	if configMap != nil {
		configMap.OwnerReferences = append(configMap.OwnerReferences, *ownerRef)

		configMap, err = c.kubeClient.CoreV1().ConfigMaps(owner.GetNamespace()).Create(configMap)
		if err != nil {
			return nil, err
		}
		defer func() {
			if configMap != nil && cleanUpConfigMap {
				if err := c.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(configMap.Name, &metav1.DeleteOptions{}); err != nil {
					logger.Warnf("Could not delete config map %v/%v", configMap.Namespace, configMap.Name)
				}
			}
		}()
	}

	if err := c.expectations.ExpectCreations(ownerKey, 1); err != nil {
		return nil, err
	}

	newJob, err := c.kubeClient.BatchV1().Jobs(owner.GetNamespace()).Create(job)
	if err != nil {
		// If an error occurred creating the job, remove expectation
		c.expectations.CreationObserved(ownerKey)
		return nil, err
	}

	cleanUpConfigMap = false

	return newJob, nil
}

func (c *jobControl) deleteOldJobs(ownerKey string, oldJobs []*kbatch.Job, logger log.FieldLogger) error {
	keysToDelete := []string{}
	for _, job := range oldJobs {
		keysToDelete = append(keysToDelete, jobKey(job))
	}
	if err := c.expectations.ExpectDeletions(ownerKey, keysToDelete); err != nil {
		return err
	}

	logger.Infof("Deleting old jobs: %v", keysToDelete)

	errCh := make(chan error, len(oldJobs))

	var wg sync.WaitGroup
	wg.Add(len(oldJobs))
	for i, ng := range oldJobs {
		go func(ix int, job *kbatch.Job) {
			defer wg.Done()
			if err := c.kubeClient.BatchV1().Jobs(job.Namespace).Delete(job.Name, &metav1.DeleteOptions{}); err != nil {
				// Decrement the expected number of deletes because the informer won't observe this deletion
				jobKey := keysToDelete[ix]
				logger.Infof("Failed to delete %v, decrementing expectations", jobKey)
				c.expectations.DeletionObserved(ownerKey, jobKey)
				errCh <- err
			}
		}(i, ng)
	}
	wg.Wait()

	select {
	case err := <-errCh:
		// all errors have been reported before and they're likely to be the same, so we'll only return the first one we hit.
		if err != nil {
			return err
		}
	default:
	}

	return nil
}

func jobOwnerGeneration(job *kbatch.Job) int64 {
	if job.Annotations == nil {
		return 0
	}
	generationStr, ok := job.Annotations[clusteroperator.OwnerGenerationAnnotation]
	if !ok {
		return 0
	}
	generation, err := strconv.ParseInt(generationStr, 10, 64)
	if err != nil {
		return 0
	}
	return generation
}

func jobKey(job *kbatch.Job) string {
	return fmt.Sprintf("%s/%s", job.Namespace, job.Name)
}

func loggerForJob(logger log.FieldLogger, job *kbatch.Job) log.FieldLogger {
	return logger.WithFields(log.Fields{"job": job.Name, "namespace": job.Namespace})
}

func loggerForOwner(logger log.FieldLogger, owner metav1.Object) log.FieldLogger {
	return logger.WithFields(log.Fields{"owner": owner.GetName(), "namespace": owner.GetNamespace()})
}

func handleError(err error, logger log.FieldLogger) {
	log.Error(err)
	utilruntime.HandleError(err)
}
