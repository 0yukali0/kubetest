package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	UpdateMonitors  = make([]*monitor.Monitor, DeploymentNum)
	Monitors        = make([]*monitor.Monitor, DeploymentNum)
	MonitorID       = 1
)

type Timestamps struct {
	StartTime      time.Time `json: start`
	InnerStartTime time.Time `json: innerStart`
	Distribution   []int     `json: distribution`
}

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
		fmt.Printf("Starting %s via scheduler %s\n", AppName, schedulerName)
		beginTime := time.Now().Truncate(time.Second)
		// YK need apisever register deploymnet first.
		if isYK {
			plsScaleDownScheduler()
			// create deployment and pending
			for MonitorID = 1; MonitorID <= DeploymentNum; MonitorID++ {
				target := fmt.Sprintf("%s%d", AppName, MonitorID)
				deployment := cache.KubeDeployment{}.WithSchedulerName(schedulerName).WithAppName(
					target).WithPodNum(int32(PodNum)).Build()
				kubeclient.CreateDeployment(common.Namespace, deployment)
			}
			//Wait for scale up YK
			beginTime = time.Now().Truncate(time.Second)
			plsScaleUpScheduler(&beginTime)
		}

		wg = &sync.WaitGroup{}
		wg.Add(DeploymentNum)
		// Start to monitor
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
		// log distribution
		WriteDistributionFile(schedulerName, beginTime, beginTime, podStartTimeDistribution)
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
		if result[2] == 0 {
			YKStop = true
		}
		fmt.Println("Pls scale down YK scheduler")
		time.Sleep(1000000000)
	}
}

func plsScaleUpScheduler(beginTime *time.Time) {

	for YKStop := false; !YKStop; {
		result := collector.CollectDeploymentMetrics(common.YSConfigMapNamespace, common.YKDeploymentName)
		if result[2] == 1 {
			setYKStartTime(beginTime)
			break
		}
		fmt.Println("Pls scale up YK scheduler")
		*beginTime = time.Now().Truncate(time.Second)
		fmt.Printf("Time update:%v\n", beginTime)
	}
}

func checkDeploymentUpdate(target string) {
	for updateOk := false; !updateOk; {
		result := collector.CollectDeploymentMetrics(common.YSConfigMapNamespace, common.YKDeploymentName)
		if result[1] == PodNum {
			break
		}
		fmt.Println("Waiting update")
		time.Sleep(1000000000)
	}
}

func setYKStartTime(beginTime *time.Time) {
	targetMap := map[string]string{cache.KeyApp: common.YSName}
	podStartTimes := collector.CollectPodInfo(common.YSConfigMapNamespace,
		kubeclient.GetListOptions(targetMap), collector.ParsePodStartTime)
	if columns, ok := podStartTimes[0].([]interface{}); ok {
		if startTime, ok := columns[0].(time.Time); ok {
			fmt.Printf("time is %v\n", startTime)
			*beginTime = startTime
		}
	}
	fmt.Printf("%v", podStartTimes)
	time.Sleep(3000000000)
}

func WriteDistributionFile(schedulerName string, ReadyTime time.Time,
	beginTime time.Time, distribution []int) {
	data := Timestamps{}
	file, _ := json.MarshalIndent(data, "", " ")
	path := fmt.Sprintf("/tmp/%s.log", schedulerName)
	_ = ioutil.WriteFile(path, file, 0644)
}
