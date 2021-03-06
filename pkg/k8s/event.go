package k8s

import (
	"github.com/go-logr/logr"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	rsv1 "redis-sentinel/api/v1"
)

// Event the client that push event to kubernetes
type Event interface {
	// NewSlaveAdd event ClusterScaling
	NewSlaveAdd(object runtime.Object, message string)
	// SlaveRemove event ClusterScalingDown
	SlaveRemove(object runtime.Object, message string)
	// CreateCluster event ClusterCreating
	CreateCluster(object runtime.Object)
	// UpdateCluster event ClusterUpdating
	UpdateCluster(object runtime.Object, message string)
	// UpgradedCluster event ClusterUpgrading
	UpgradedCluster(object runtime.Object, message string)
	// EnsureCluster event Ensure
	EnsureCluster(object runtime.Object)
	// CheckCluster event CheckAndHeal
	CheckCluster(object runtime.Object)
	// FailedCluster event ClusterFailed
	FailedCluster(object runtime.Object, message string)
	// HealthCluster event ClusterHealthy
	HealthCluster(object runtime.Object)
}

// EventOption is the Event client interface implementation that using API calls to kubernetes.
type EventOption struct {
	eventsCli record.EventRecorder
	logger    logr.Logger
}

// NewEvent returns a new Event client
func NewEvent(eventCli record.EventRecorder, logger logr.Logger) Event {
	return &EventOption{
		eventsCli: eventCli,
		logger:    logger,
	}
}

// NewSlaveAdd implement the Event.Interface
func (e *EventOption) NewSlaveAdd(object runtime.Object, message string) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionScaling), message)
}

// SlaveRemove implement the Event.Interface
func (e *EventOption) SlaveRemove(object runtime.Object, message string) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionScalingDown), message)
}

// CreateCluster implement the Event.Interface
func (e *EventOption) CreateCluster(object runtime.Object) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionCreating), "Bootstrap redis cluster")
}

// UpdateCluster implement the Event.Interface
func (e *EventOption) UpdateCluster(object runtime.Object, message string) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionUpdating), message)
}

// UpgradedCluster implement the Event.Interface
func (e *EventOption) UpgradedCluster(object runtime.Object, message string) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionUpgrading), message)
}

// EnsureCluster implement the Event.Interface
func (e *EventOption) EnsureCluster(object runtime.Object) {
	e.eventsCli.Event(object, v1.EventTypeNormal, "Ensure", "Makes sure of redis cluster ready")
}

// CheckCluster implement the Event.Interface
func (e *EventOption) CheckCluster(object runtime.Object) {
	e.eventsCli.Event(object, v1.EventTypeNormal, "CheckAndHeal", "Check and heal the redis cluster problems")
}

// FailedCluster implement the Event.Interface
func (e *EventOption) FailedCluster(object runtime.Object, message string) {
	e.eventsCli.Event(object, v1.EventTypeWarning, string(rsv1.ClusterConditionFailed), message)
}

// HealthCluster implement the Event.Interface
func (e *EventOption) HealthCluster(object runtime.Object) {
	e.eventsCli.Event(object, v1.EventTypeNormal, string(rsv1.ClusterConditionHealthy), "Redis cluster is healthy")
}
