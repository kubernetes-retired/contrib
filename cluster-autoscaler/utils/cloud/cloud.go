package cloud

import (
	"k8s.io/contrib/cluster-autoscaler/config"
)

type Manager interface {
	GetScalingGroupSize(*config.ScalingConfig) (int64, error)
	SetScalingGroupSize(*config.ScalingConfig, int64) error
	DeleteInstances([]*config.InstanceConfig) error
	GetScalingGroupForInstance(*config.InstanceConfig) (*config.ScalingConfig, error)
}
