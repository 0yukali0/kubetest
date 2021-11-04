package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"time"

	"github.com/TaoYang526/kubetest/pkg/common"
	"github.com/TaoYang526/kubetest/pkg/painter"
)

const (
	ChartTitle  = "Scheduling Throughput"
	ChartXLabel = "Seconds"
	ChartYLabel = "Number of Pods"
)

type Timestamps struct {
	StartTime      time.Time `json: start`
	InnerStartTime time.Time `json: innerStart`
	Distribution   []int     `json: distribution`
}

func main() {
	dataMap := make(map[string][]int, 2)
	for _, schedulerName := range common.SchedulerNames {
		path := fmt.Sprintf("/tmp/log_%s.json", schedulerName)
		data := Timestamps{}
		file, _ := ioutil.ReadFile(path)
		_ = json.Unmarshal([]byte(file), &data)
		distribution := data.Distribution
		var shiftDistribution []int
		if !data.StartTime.Equal(data.InnerStartTime) {
			seconds := int(math.Ceil(data.InnerStartTime.Sub(data.StartTime).Seconds()))
			if seconds < 0 {
				fmt.Println("Negative time: inner should be slow than k8s timestamp")
				// go to paint
				break
			}
			// check shift safely
			safe := true
			for i := 0; i < seconds; i++ {
				if distribution[i] != 0 {
					fmt.Println("Negative time: inner should be slow than k8s timestamp")
					safe = false
					break
				}
			}
			max := len(distribution) - 1
			if safe {
				shiftDistribution = make([]int, len(distribution)-seconds)
			} else {
				break
			}
			for i := seconds; i < max; i++ {
				shiftDistribution[i-seconds] = distribution[i]
			}
			// TODO shift k8stime to inner start time
		}
		dataMap[common.SchedulerAlias[schedulerName]] = shiftDistribution
	}
	linePoints := painter.GetLinePoints(dataMap)
	chart := &painter.Chart{
		Title:      ChartTitle,
		XLabel:     ChartXLabel,
		YLabel:     ChartYLabel,
		Width:      common.ChartWidth,
		Height:     common.ChartHeight,
		LinePoints: linePoints,
		SvgFile:    common.ChartSavePath + "json_throughput.svg",
	}
	painter.DrawChart(chart)
}
