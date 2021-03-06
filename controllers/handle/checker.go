package handle

import (
	"errors"
	"fmt"
	"time"

	rsv1 "redis-sentinel/api/v1"
	"redis-sentinel/controllers/clustercache"
	"redis-sentinel/pkg/util"
)

const (
	checkInterval  = 5 * time.Second
	timeOut        = 30 * time.Second
	NeedRequeueMsg = "need requeue"
)

var (
	needRequeueErr = errors.New(NeedRequeueMsg)
)

// CheckAndHeal Check the health of the cluster and heal,
// Waiting Number of ready redis is equal as the set on the RedisCluster spec
// Waiting Number of ready sentinel is equal as the set on the RedisCluster spec
// Check only one master
// Number of redis master is 1
// All redis slaves have the same master
// Set Custom Redis config
// All sentinels points to the same redis master
// Sentinel has not death nodes
// Sentinel knows the correct slave number
func (rsh *RedisSentinelHandler) CheckAndHeal(meta *clustercache.Meta) error {
	if err := rsh.RsChecker.CheckRedisNumber(meta.Obj); err != nil {
		rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).V(2).Info("number of redis mismatch, this could be for a change on the statefulset")
		rsh.EventsCli.UpdateCluster(meta.Obj, "wait for all redis server start")
		return needRequeueErr
	}
	if err := rsh.RsChecker.CheckSentinelNumber(meta.Obj); err != nil {
		rsh.EventsCli.FailedCluster(meta.Obj, err.Error())
		return nil
	}

	nMasters, err := rsh.RsChecker.GetNumberMasters(meta.Obj, meta.Auth)
	if err != nil {
		return err
	}
	switch nMasters {
	case 0:
		rsh.EventsCli.UpdateCluster(meta.Obj, "set master")
		rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).V(2).Info("no master find, fixing...")
		redisesIP, err := rsh.RsChecker.GetRedisesIPs(meta.Obj, meta.Auth)
		if err != nil {
			return err
		}
		if len(redisesIP) == 1 {
			if err := rsh.RsHealer.MakeMaster(redisesIP[0], meta.Auth); err != nil {
				return err
			}
			break
		}
		minTime, err := rsh.RsChecker.GetMinimumRedisPodTime(meta.Obj)
		if err != nil {
			return err
		}
		rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).Info(fmt.Sprintf("time %.f more than expected. Not even one master, fixing...", minTime.Round(time.Second).Seconds()))
		if err := rsh.RsHealer.SetOldestAsMaster(meta.Obj, meta.Auth); err != nil {
			return err
		}
	case 1:
		break
	default:
		return errors.New("more than one master, fix manually")
	}

	master, err := rsh.RsChecker.GetMasterIP(meta.Obj, meta.Auth)
	if err != nil {
		return err
	}
	if err := rsh.RsChecker.CheckAllSlavesFromMaster(master, meta.Obj, meta.Auth); err != nil {
		rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).Info(err.Error())
		if err := rsh.RsHealer.SetMasterOnAll(master, meta.Obj, meta.Auth); err != nil {
			return err
		}
	}

	if err = rsh.setRedisConfig(meta); err != nil {
		return err
	}

	sentinels, err := rsh.RsChecker.GetSentinelsIPs(meta.Obj)
	if err != nil {
		return err
	}
	for _, sip := range sentinels {
		if err := rsh.RsChecker.CheckSentinelMonitor(sip, master, meta.Auth); err != nil {
			rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).Info(err.Error())
			if err := rsh.RsHealer.NewSentinelMonitor(sip, master, meta.Obj, meta.Auth); err != nil {
				return err
			}
		}
	}
	for _, sip := range sentinels {
		if err := rsh.RsChecker.CheckSentinelSlavesNumberInMemory(sip, meta.Obj, meta.Auth); err != nil {
			rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).
				Info("restoring sentinel ...", "sentinel", sip, "reason", err.Error())
			if err := rsh.RsHealer.RestoreSentinel(sip, meta.Auth); err != nil {
				return err
			}
			if err := rsh.waitRestoreSentinelSlavesOK(sip, meta.Obj, meta.Auth); err != nil {
				rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).Info(err.Error())
				return err
			}
		}
	}
	for _, sip := range sentinels {
		if err := rsh.RsChecker.CheckSentinelNumberInMemory(sip, meta.Obj, meta.Auth); err != nil {
			rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).
				Info("restoring sentinel ...", "sentinel", sip, "reason", err.Error())
			if err := rsh.RsHealer.RestoreSentinel(sip, meta.Auth); err != nil {
				return err
			}
		}
	}

	if err = rsh.setSentinelConfig(meta, sentinels); err != nil {
		return err
	}

	return nil
}

func (rsh *RedisSentinelHandler) setRedisConfig(meta *clustercache.Meta) error {
	redises, err := rsh.RsChecker.GetRedisesIPs(meta.Obj, meta.Auth)
	if err != nil {
		return err
	}
	for _, rip := range redises {
		if err := rsh.RsChecker.CheckRedisConfig(meta.Obj, rip, meta.Auth); err != nil {
			rsh.Logger.WithValues("namespace", meta.Obj.Namespace, "name", meta.Obj.Name).Info(err.Error())
			rsh.EventsCli.UpdateCluster(meta.Obj, "set custom config for redis server")
			if err := rsh.RsHealer.SetRedisCustomConfig(rip, meta.Obj, meta.Auth); err != nil {
				return err
			}
		}
	}
	return nil
}

// TODO do as set redis config
func (rsh *RedisSentinelHandler) setSentinelConfig(meta *clustercache.Meta, sentinels []string) error {
	if meta.State == clustercache.Check {
		return nil
	}

	for _, sip := range sentinels {
		if err := rsh.RsHealer.SetSentinelCustomConfig(sip, meta.Obj, meta.Auth); err != nil {
			return err
		}
	}
	return nil
}

func (rsh *RedisSentinelHandler) waitRestoreSentinelSlavesOK(sentinel string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	timer := time.NewTimer(timeOut)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("wait for resetore sentinel slave timeout")
		default:
			if err := rsh.RsChecker.CheckSentinelSlavesNumberInMemory(sentinel, rs, auth); err != nil {
				rsh.Logger.WithValues("namespace", rs.Namespace, "name", rs.Name).Info(err.Error())
				time.Sleep(checkInterval)
			} else {
				return nil
			}
		}
	}
}
