package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/TaoYang526/kubetest/pkg/cache"
	"github.com/TaoYang526/kubetest/pkg/collector"
	"github.com/TaoYang526/kubetest/pkg/common"
	"github.com/TaoYang526/kubetest/pkg/kubeclient"
	"github.com/TaoYang526/kubetest/pkg/monitor"
	"github.com/TaoYang526/kubetest/pkg/painter"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppName     = "throughput.test"
	ChartTitle  = "Scheduling Throughput"
	ChartXLabel = "Seconds"
	ChartYLabel = "Number of Pods"
)

var (
	TotolPodNum     = 1000
	DeploymentNum   = 10
	PodNum          = TotolPodNum / DeploymentNum
	SelectPodLabels = map[string]string{cache.KeyApp: AppName}
	Monitors        = make([]*monitor.Monitor, DeploymentNum)
	MonitorID       = 1
)

func main() {
	// make sure all related pods are cleaned up
	wg := &sync.WaitGroup{}
	/*wg.Add(DeploymentNum)
	for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
		monitor.WaitUtilAllMetricsAreCleanedUp(wg, collectDeploymentMetrics, MonitorID)
	}
	wg.Done()*/

	dataMap := make(map[string][]int, 2)
	for _, schedulerName := range common.SchedulerNames {
		fmt.Printf("Starting %s via scheduler %s\n", AppName, schedulerName)
		// create deployment
		beginTime := time.Now().Truncate(time.Second)
		wg = &sync.WaitGroup{}
		wg.Add(DeploymentNum)
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
			deployment := cache.KubeDeployment{}.WithSchedulerName(schedulerName).WithAppName(
				target).WithPodNum(int32(PodNum)).Build()
			kubeclient.CreateDeployment(common.Namespace, deployment)
			// start monitor
			createMonitor := &monitor.Monitor{
				Name:           AppName + " create-monitor" + target,
				Num:            MonitorID,
				Interval:       1,
				CollectMetrics: collectDeploymentMetrics,
				SkipSameMerics: true,
				StopTrigger: func(m *monitor.Monitor) bool {
					lastCp := m.GetLastCheckPoint()
					actualNum := lastCp.MetricValues[2]
					fmt.Printf("Montir %d actual ready:%d\n", m.Num, actualNum)
					if actualNum == PodNum {
						// stop monitor when readyReplicas equals PodNum
						return true
					}
					return false
				},
			}
			createMonitor.SetWG(wg)
			Monitors[MonitorID-1] = createMonitor
			Monitors[MonitorID-1].Start()
		}
		// wait all deployments running
		wg.Wait()
		// calculate distribution of pod start times
		endTime := time.Now()
		var lists []*v1.ListOptions = make([]*v1.ListOptions, DeploymentNum)
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
			targetMap := map[string]string{cache.KeyApp: target}
			lists[MonitorID-1] = kubeclient.GetListOptions(targetMap)
		}
		podStartTimes := collector.CollectPodInfoWithID(DeploymentNum, common.Namespace,
			lists, collector.ParsePodStartTime)
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
		wg = &sync.WaitGroup{}
		wg.Add(DeploymentNum)
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			monitor.WaitUtilAllMetricsAreCleanedUp(wg, collectDeploymentMetrics, MonitorID)
		}
		wg.Wait()
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

func collectDeploymentMetrics(id int) []int {
	target := fmt.Sprintf("%s%d", AppName, id)
	fmt.Printf("%d Assign %s\n", id, target)
	return collector.CollectDeploymentMetrics(common.Namespace, target)
}
