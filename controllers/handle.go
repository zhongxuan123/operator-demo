package controllers

import (
	"fmt"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "redis-sentinel/api/v1"
	"redis-sentinel/controllers/clustercache"
	"redis-sentinel/pkg/k8s"
	"redis-sentinel/pkg/metrics"
	"redis-sentinel/pkg/util"
	"redis-sentinel/service"
	)

var (
	defaultLabels = map[string]string{
		v1.LabelManagedByKey:v1.OperatorName,
	}
)

// RedisClusterHandler is the RedisCluster handler. This handler will create the required
// resources that a RedisCluster needs.
type RedisSentinelHandler struct {
	k8sServices k8s.Services
	rcService   service.RedisSentinelClient
	rcChecker   service.RedisClusterCheck
	rcHealer    service.RedisClusterHeal
	MetaCache   *clustercache.MetaMap
	eventsCli   k8s.Event
	logger      logr.Logger
}

// Do will ensure the RedisCluster is in the expected state and update the RedisCluster status.
func (rsh *RedisSentinelHandler) Do(rc *v1.RedisSentinel) error {
	rsh.logger.WithValues("namespace", rc.Namespace, "name", rc.Name).Info("handler doing")
	if err := rc.Validate(); err != nil {
		metrics.ClusterMetrics.SetClusterError(rc.Namespace, rc.Name)
		return err
	}

	// diff new and new RedisCluster, then update status
	meta := rsh.MetaCache.Cache(rc)
	rsh.logger.WithValues("namespace", rc.Namespace, "name", rc.Name).V(3).
		Info(fmt.Sprintf("meta status:%s, mes:%s, state:%s", meta.Status, meta.Message, meta.State))
	rsh.updateStatus(meta)

	// Create owner refs so the objects manager by this handler have ownership to the
	// received rc.
	oRefs := rsh.createOwnerReferences(rc)

	// Create the labels every object derived from this need to have.
	labels := rsh.getLabels(rc)

	rsh.logger.WithValues("namespace", rc.Namespace, "name", rc.Name).V(2).Info("Ensure...")
	rsh.eventsCli.EnsureCluster(rc)
	if err := rsh.Ensure(meta.Obj, labels, oRefs); err != nil {
		rsh.eventsCli.FailedCluster(rc, err.Error())
		rc.Status.SetFailedCondition(err.Error())
		rsh.k8sServices.UpdateCluster(rc.Namespace, rc)
		metrics.ClusterMetrics.SetClusterError(rc.Namespace, rc.Name)
		return err
	}

	rsh.logger.WithValues("namespace", rc.Namespace, "name", rc.Name).V(2).Info("CheckAndHeal...")
	rsh.eventsCli.CheckCluster(rc)
	if err := rsh.CheckAndHeal(meta); err != nil {
		metrics.ClusterMetrics.SetClusterError(rc.Namespace, rc.Name)
		if err.Error() != needRequeueMsg {
			rsh.eventsCli.FailedCluster(rc, err.Error())
			rc.Status.SetFailedCondition(err.Error())
			rsh.k8sServices.UpdateCluster(rc.Namespace, rc)
			return err
		}
		// if user delete statefulset or deployment, set status
		status := rc.Status.Conditions
		if len(status) > 0 && status[0].Type == v1.ClusterConditionHealthy {
			rsh.eventsCli.CreateCluster(rc)
			rc.Status.SetCreateCondition("redis server or sentinel server be removed by user, restart")
			rsh.k8sServices.UpdateCluster(rc.Namespace, rc)
		}
		return err
	}

	rsh.logger.WithValues("namespace", rc.Namespace, "name", rc.Name).V(2).Info("SetReadyCondition...")
	rsh.eventsCli.HealthCluster(rc)
	rc.Status.SetReadyCondition("Cluster ok")
	rsh.k8sServices.UpdateCluster(rc.Namespace, rc)
	metrics.ClusterMetrics.SetClusterOK(rc.Namespace, rc.Name)

	return nil
}

func (rsh *RedisSentinelHandler) updateStatus(meta *clustercache.Meta) {
	rc := meta.Obj
	if meta.State != clustercache.Check {
		// Password change is not allowed
		//rc.Spec.Redis.Password = rc.Spec.Redis.Password
		switch meta.Status {
		case v1.ClusterConditionCreating:
			rsh.eventsCli.CreateCluster(rc)
			rc.Status.SetCreateCondition(meta.Message)
		case v1.ClusterConditionScaling:
			rsh.eventsCli.NewSlaveAdd(rc, meta.Message)
			rc.Status.SetScalingUpCondition(meta.Message)
		case v1.ClusterConditionScalingDown:
			rsh.eventsCli.SlaveRemove(rc, meta.Message)
			rc.Status.SetScalingDownCondition(meta.Message)
		case v1.ClusterConditionUpgrading:
			rsh.eventsCli.UpdateCluster(rc, meta.Message)
			rc.Status.SetUpgradingCondition(meta.Message)
		default:
			rsh.eventsCli.UpdateCluster(rc, meta.Message)
			rc.Status.SetUpdatingCondition(meta.Message)
		}
		rsh.k8sServices.UpdateCluster(rc.Namespace, rc)
	}
}

// getLabels merges all the labels (dynamic and operator static ones).
func (rsh *RedisSentinelHandler) getLabels(rs *v1.RedisSentinel) map[string]string {
	dynLabels := map[string]string{
		v1.LabelNameKey: fmt.Sprintf("%s%c%s", rs.Namespace, '_', rs.Name),
	}
	return util.MergeLabels(defaultLabels, dynLabels, rs.Labels)
}

func (rsh *RedisSentinelHandler) createOwnerReferences(rs *v1.RedisSentinel) []metav1.OwnerReference {
	rsvk := v1.VersionKind(v1.Kind)
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(rs, rsvk),
	}
}
