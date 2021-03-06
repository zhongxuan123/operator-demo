package v1

import (
	"errors"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	maxNameLength = 48

	defaultRedisNumber    = 3
	defaultSentinelNumber = 3
	defaultRedisImage     = "redis:5.0.4-alpine"

	defaultSlavePriority = "1"
)

var (
	defaultSentinelCustomConfig = []string{"down-after-milliseconds 5000", "failover-timeout 10000"}
)

// Validate set the values by default if not defined and checks if the values given are valid
func (rc *RedisSentinel) Validate() error {
	if len(rc.Name) > maxNameLength {
		return fmt.Errorf("name length can't be higher than %d", maxNameLength)
	}

	if rc.Spec.Size == 0 {
		rc.Spec.Size = defaultRedisNumber
	} else if rc.Spec.Size < defaultRedisNumber {
		return errors.New("number of redis in spec is less than the minimum")
	}

	if rc.Spec.Sentinel.Replicas == 0 {
		rc.Spec.Sentinel.Replicas = defaultSentinelNumber
	} else if rc.Spec.Sentinel.Replicas < defaultSentinelNumber {
		return errors.New("number of sentinels in spec is less than the minimum")
	}

	if rc.Spec.Image == "" {
		rc.Spec.Image = defaultRedisImage
	}

	if rc.Spec.Sentinel.Image == "" {
		rc.Spec.Sentinel.Image = defaultRedisImage
	}

	if rc.Spec.Sentinel.Resources.Size() == 0 {
		rc.Spec.Sentinel.Resources = defaultSentinelResource()
	}

	if rc.Spec.Config == nil {
		rc.Spec.Config = make(map[string]string)
	}

	// https://github.com/ucloud/redis-operator/issues/6
	rc.Spec.Config["slave-priority"] = defaultSlavePriority

	if !rc.Spec.DisablePersistence {
		enablePersistence(rc.Spec.Config)
	} else {
		disablePersistence(rc.Spec.Config)
	}

	return nil
}

func enablePersistence(config map[string]string) {
	setConfigMapIfNotExist("appendonly", "yes", config)
	setConfigMapIfNotExist("auto-aof-rewrite-min-size", "536870912", config)
	setConfigMapIfNotExist("auto-aof-rewrite-percentage", "100", config)
	setConfigMapIfNotExist("repl-backlog-size", "62914560", config)
	setConfigMapIfNotExist("repl-diskless-sync", "yes", config)
	setConfigMapIfNotExist("aof-load-truncated", "yes", config)
	setConfigMapIfNotExist("stop-writes-on-bgsave-error", "no", config)
	setConfigMapIfNotExist("save", "900 1 300 10", config)
}

func setConfigMapIfNotExist(key, value string, config map[string]string) {
	if _, ok := config[key]; !ok {
		config[key] = value
	}
}

func disablePersistence(config map[string]string) {
	config["appendonly"] = "no"
	config["save"] = ""
}

func defaultSentinelResource() v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("20m"),
			v1.ResourceMemory: resource.MustParse("16Mi"),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("100m"),
			v1.ResourceMemory: resource.MustParse("60Mi"),
		},
	}
}
