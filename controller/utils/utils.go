package utils

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
)

// TaskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type TaskQueue struct {
	// queue is the work queue the worker polls
	queue workqueue.RateLimitingInterface
	// sync is called for each item in the queue
	sync func(string)
	// workerDone is closed when the worker exits
	workerDone chan struct{}
	// key func is to define the key to be used by cache
	keyFunc func(obj interface{}) (string, error)
}

// Enqueue enqueues ns/name of the given api object in the task queue.
func (t *TaskQueue) Enqueue(obj interface{}) {
	if key, ok := obj.(string); ok {
		t.queue.Add(key)
	} else {
		key, err := t.keyFunc(obj)
		if err != nil {
			logrus.Infof("could not get key for object %+v: %v", obj, err)
			return
		}
		t.queue.Add(key)
	}
}

func (t *TaskQueue) Requeue(key string, err error) {
	utilruntime.HandleError(errors.Wrap(err, fmt.Sprintf("Sync %q failed", key)))
	t.queue.AddRateLimited(key)
}

// worker processes work in the queue through sync.
func (t *TaskQueue) worker() {
	for {
		key, quit := t.queue.Get()
		if quit {
			close(t.workerDone)
			return
		}
		logrus.Debugf("syncing %v", key)
		t.sync(key.(string))
		t.queue.Done(key)
	}
}

// Shutdown shuts down the work queue and waits for the worker to ACK
func (t *TaskQueue) Shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(name string, keyFunc func(obj interface{}) (string, error), syncFn func(string)) *TaskQueue {
	return &TaskQueue{
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		sync:       syncFn,
		keyFunc:    keyFunc,
		workerDone: make(chan struct{}),
	}
}

func (t *TaskQueue) Run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

// CreateClusterClient creates a new client for the cluster
func CreateClusterClient(host string, token string, caCert string) (*kubernetes.Clientset, error) {
	var cfg *rest.Config

	cfg = &rest.Config{
		Host:        host,
		BearerToken: token,
	}
	cfg.TLSClientConfig.CAData = []byte(caCert)

	return kubernetes.NewForConfig(cfg)
}
