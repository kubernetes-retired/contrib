package factory

import (
	"k8s.io/contrib/cluster-autoscaler/expander"
	"k8s.io/contrib/cluster-autoscaler/expander/random"
	"k8s.io/contrib/cluster-autoscaler/expander/mostpods"
	"k8s.io/contrib/cluster-autoscaler/expander/waste"
)

func ExpanderStrategyFromString(expanderFlag string) expander.Strategy {
	var expanderStrategy expander.Strategy
	{
		switch expanderFlag {
		case expander.RandomExpanderName:
			expanderStrategy = random.NewStrategy()
		case expander.MostPodsExpanderName:
			expanderStrategy = mostpods.NewStrategy()
		case expander.LeastWasteExpanderName:
			expanderStrategy = waste.NewStrategy()
		}
	}
	return expanderStrategy
}
