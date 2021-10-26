package main

import (
	"fmt"
	"time"

	"github.com/TaoYang526/kubetest/pkg/cache"
	"github.com/TaoYang526/kubetest/pkg/collector"
	"github.com/TaoYang526/kubetest/pkg/common"
	"github.com/TaoYang526/kubetest/pkg/kubeclient"
	"github.com/TaoYang526/kubetest/pkg/monitor"
	"github.com/TaoYang526/kubetest/pkg/painter"
)

const (
	AppName     = "throughput.test"
	ChartTitle  = "Scheduling Throughput"
	ChartXLabel = "Seconds"
	ChartYLabel = "Number of Pods"
)

var (
	TotolPodNum     = 50000
	DeploymentNum   = 10
	PodNum          = TotolPodNum / DeploymentNum
	SelectPodLabels = map[string]string{cache.KeyApp: AppName}
	Monitors        = make([]*monitor.Monitor, DeploymentNum)
	MonitorID       = 1
)

func main() {
	// make sure all related pods are cleaned up
	monitor.WaitUtilAllMetricsAreCleanedUp(collectDeploymentMetrics)

	dataMap := make(map[string][]int, 2)
	for _, schedulerName := range common.SchedulerNames {
		fmt.Printf("Starting %s via scheduler %s\n", AppName, schedulerName)
		// create deployment
		beginTime := time.Now().Truncate(time.Second)
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
			deployment := cache.KubeDeployment{}.WithSchedulerName(schedulerName).WithAppName(
				target).WithPodNum(int32(PodNum)).Build()
			kubeclient.CreateDeployment(common.Namespace, deployment)
			// start monitor
			createMonitor := &monitor.Monitor{
				Name:           AppName + " create-monitor" + fmt.Sprintf("%d", MonitorID),
				Interval:       15,
				CollectMetrics: collectDeploymentMetrics,
				SkipSameMerics: true,
				StopTrigger: func(m *monitor.Monitor) bool {
					lastCp := m.GetLastCheckPoint()
					if lastCp.MetricValues[2] == PodNum {
						// stop monitor when readyReplicas equals PodNum
						return true
					}
					return false
				},
			}
			Monitors[MonitorID-1] = createMonitor
			createMonitor.Start()
		}
		// wait all deployments running
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			// wait util this deployment is running successfully
			Monitors[MonitorID-1].WaitForStopped()
		}
		// calculate distribution of pod start times
		endTime := time.Now()
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {

		}
		var podStartTimes []interface{}
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
			targetMap := map[string]string{cache.KeyApp: target}
			podStartTime := collector.CollectPodInfo(common.Namespace,
				kubeclient.GetListOptions(targetMap), collector.ParsePodStartTime)
			podStartTimes = append(podStartTimes, podStartTime)
		}
		/*
			podStartTimes := collector.CollectPodInfo(common.Namespace,
				kubeclient.GetListOptions(SelectPodLabels), collector.ParsePodStartTime)
		*/
		podStartTimeDistribution := collector.AnalyzeTimeDistribution(beginTime, endTime, podStartTimes)
		fmt.Printf("Distribution of pod start times: %v, seconds: %d beginTime: %v, endTime: %v \n",
			podStartTimeDistribution, len(podStartTimeDistribution), beginTime, endTime)

		// Save checkpoints
		dataMap[common.SchedulerAlias[schedulerName]] = podStartTimeDistribution

		// delete deployment
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
			kubeclient.DeleteDeployment(common.Namespace, target)
		}
		// make sure all related pods are cleaned up
		// monitor.WaitUtilAllMetricsAreCleanedUp(collectDeploymentMetrics)
	}

	// draw chart
	linePoints := painter.GetLinePoints(dataMap)
	chart := &painter.Chart{
		Title:      ChartTitle,
		XLabel:     ChartXLabel,
		YLabel:     ChartYLabel,
		Width:      common.ChartWidth,
		Height:     common.ChartHeight,
		LinePoints: linePoints,
		SvgFile:    common.ChartSavePath + "throughput.svg",
	}
	painter.DrawChart(chart)
}

func collectDeploymentMetrics() []int {
	target := fmt.Sprintf("%s%d", AppName, MonitorID)
	return collector.CollectDeploymentMetrics(common.Namespace, target)
}
