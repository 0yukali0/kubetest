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
	dataMap := make(map[string][]int, 2)
	for _, schedulerName := range common.SchedulerNames {
		// scale down YK
		isYK := false
		if schedulerName == common.SchedulerNames[0] {
			isYK = true
		}
		if isYK {
			plsScaleDownScheduler()
			// create deployment and pending
			wg = &sync.WaitGroup{}
			wg.Add(DeploymentNum)
			for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
				target := fmt.Sprintf("%s%d", AppName, MonitorID)
				deployment := cache.KubeDeployment{}.WithSchedulerName(schedulerName).WithAppName(
					target).WithPodNum(int32(PodNum)).Build()
				kubeclient.CreateDeployment(common.Namespace, deployment)
				updateMonitor := &monitor.Monitor{
					Name:           AppName + " check-update-monitor" + target,
					Num:            MonitorID,
					Interval:       1,
					CollectMetrics: collectDeploymentMetrics,
					SkipSameMerics: true,
					StopTrigger: func(m *monitor.Monitor) bool {
						lastCp := m.GetLastCheckPoint()
						actualNum := lastCp.MetricValues[1]
						fmt.Printf("Monitor %d update:%d\n", m.Num, actualNum)
						if actualNum == PodNum {
							// stop monitor when updateReplicas equals PodNum
							return true
						}
						return false
					},
				}
				updateMonitor.Start()
			}
			wg.Wait()
			//Wait for scale up YK
			plsScaleUpScheduler()
		}

		fmt.Printf("Starting %s via scheduler %s\n", AppName, schedulerName)
		//Start to check
		beginTime := time.Now().Truncate(time.Second)
		wg = &sync.WaitGroup{}
		wg.Add(DeploymentNum)
		for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
			target := fmt.Sprintf("%s%d", AppName, MonitorID)
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
					fmt.Printf("Monitor %d ready:%d\n", m.Num, actualNum)
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
	//fmt.Printf("%d Assign %s\n", id, target)
	return collector.CollectDeploymentMetrics(common.Namespace, target)
}

func plsScaleDownScheduler() {
	for YKStop := false; !YKStop; {
		result := collector.CollectDeploymentMetrics(common.YSConfigMapNamespace, common.YKDeploymentName)
		if result[1] == 0 {
			YKStop = true
		}
		fmt.Println("Pls scale down YK scheduler")
		time.Sleep(1000000000)
	}
}

func plsScaleUpScheduler() {
	for YKStop := false; !YKStop; {
		result := collector.CollectDeploymentMetrics(common.YSConfigMapNamespace, common.YKDeploymentName)
		if result[2] == 1 {
			YKStop = true
		}
		fmt.Println("Pls scale up YK scheduler")
	}
}
