package v1
import (
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// Phase of the RF status
type Phase string

// Condition saves the state information of the redis cluster
type Condition struct {
	// Status of cluster condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string    `json:"lastUpdateTime,omitempty"`
	LastUpdateAt   time.Time `json:"-"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// ConditionType defines the condition that the RF can have
type ConditionType string

const (
	ClusterConditionAvailable   ConditionType = "Available"
	ClusterConditionHealthy     ConditionType = "Healthy"
	ClusterConditionRunning                   = "Running"
	ClusterConditionCreating                  = "Creating"
	ClusterConditionRecovering                = "Recovering"
	ClusterConditionScaling                   = "Scaling"
	ClusterConditionScalingDown               = "ScalingDown"
	ClusterConditionUpgrading                 = "Upgrading"
	ClusterConditionUpdating                  = "Updating"
	ClusterConditionFailed                    = "Failed"
)

// RedisClusterStatus defines the observed state of RedisCluster
// +k8s:openapi-gen=true
type RedisSentinelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions []Condition `json:"conditions,omitempty"`
	MasterIP   string      `json:"masterIP,omitempty"`
	SentinelIP string      `json:"sentinelIP,omitempty"`
}

func (rss *RedisSentinelStatus) DescConditionsByTime() {
	sort.Slice(rss.Conditions, func(i, j int) bool {
		return rss.Conditions[i].LastUpdateAt.After(rss.Conditions[j].LastUpdateAt)
	})
}

func (rss *RedisSentinelStatus) SetScalingUpCondition(message string) {
	c := newClusterCondition(ClusterConditionScaling, corev1.ConditionTrue, "Scaling up", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetCreateCondition(message string) {
	c := newClusterCondition(ClusterConditionCreating, corev1.ConditionTrue, "Creating", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetScalingDownCondition(message string) {
	c := newClusterCondition(ClusterConditionScaling, corev1.ConditionTrue, "Scaling down", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetUpgradingCondition(message string) {
	c := newClusterCondition(ClusterConditionUpgrading, corev1.ConditionTrue,
		"Cluster upgrading", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetUpdatingCondition(message string) {
	c := newClusterCondition(ClusterConditionUpdating, corev1.ConditionTrue,
		"Cluster updating", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetReadyCondition(message string) {
	c := newClusterCondition(ClusterConditionHealthy, corev1.ConditionTrue, "Cluster available", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) SetFailedCondition(message string) {
	c := newClusterCondition(ClusterConditionFailed, corev1.ConditionTrue,
		"Cluster failed", message)
	rss.setClusterCondition(*c)
}

func (rss *RedisSentinelStatus) ClearCondition(t ConditionType) {
	pos, _ := getClusterCondition(rss, t)
	if pos == -1 {
		return
	}
	rss.Conditions = append(rss.Conditions[:pos], rss.Conditions[pos+1:]...)
}

func (rss *RedisSentinelStatus) setClusterCondition(c Condition) {
	pos, cp := getClusterCondition(rss, c.Type)
	if cp != nil &&
		cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
		now := time.Now()
		nowString := now.Format(time.RFC3339)
		rss.Conditions[pos].LastUpdateAt = now
		rss.Conditions[pos].LastUpdateTime = nowString
		return
	}

	if cp != nil {
		rss.Conditions[pos] = c
	} else {
		rss.Conditions = append(rss.Conditions, c)
	}
}

func getClusterCondition(rss *RedisSentinelStatus, t ConditionType) (int, *Condition) {
	for i, c := range rss.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func newClusterCondition(condType ConditionType, status corev1.ConditionStatus, reason, message string) *Condition {
	now := time.Now()
	nowString := now.Format(time.RFC3339)
	return &Condition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     nowString,
		LastUpdateAt:       now,
		LastTransitionTime: nowString,
		Reason:             reason,
		Message:            message,
	}
}
