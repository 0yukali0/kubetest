package common

import "gonum.org/v1/plot/vg"

const (
	// constants for testing environment
	QueueName        = "root.sandbox"
	Namespace        = "default"
	HollowNodePrefix = "hollow-"

	// constants for yunikorn scheduler
	YSConfigMapNamespace            = "yunikorn"
	YSConfigMapName                 = "yunikorn-configs"
	YSConfigMapQueuesYamlKey        = "queues.yaml"
	YSConfigMapQueuesResourceMemKey = "memory"
	YSName                          = "yunikorn"
	DeployReady                     = 1

	// constants for chart
	ChartWidth    = 6 * vg.Inch
	ChartHeight   = 6 * vg.Inch
	ChartSavePath = "/tmp/"
)

var (
	SchedulerNames = []string{YSName, ""}
	SchedulerAlias = map[string]string{YSName: YSName, "": "k8s"}
)
