package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/TaoYang526/kubetest/pkg/cache"
)

type Monitor struct {
	Name           string
	Num            int
	NameSpace      string
	Interval       int //in seconds
	StopTrigger    func(m *Monitor) bool
	CollectMetrics func(id int) []int
	SkipSameMerics bool

	checkpoints []*cache.Checkpoint
	startTime   time.Time
	stopTime    time.Time
	stopChan    chan bool
	wg          *sync.WaitGroup
}

func (m *Monitor) GetLastCheckPoint() *cache.Checkpoint {
	return m.checkpoints[len(m.checkpoints)-1]
}

func (m *Monitor) GetCheckPoints() []*cache.Checkpoint {
	return m.checkpoints
}

func (m *Monitor) Start() {
	m.stopChan = make(chan bool)
	m.startTime = time.Now()
	firstCP := &cache.Checkpoint{
		Time:    m.startTime,
		Seconds: 0,
	}
	m.checkpoints = append(m.checkpoints, firstCP)
	// m.wg.Add(1)
	go func(m *Monitor) {
		nextSeconds := 0
		lastCp := m.checkpoints[0]
	LOOP:
		for {
			select {
			case <-m.stopChan:
				fmt.Printf("Monitor[%s] is exiting when receiving stop signal\n", m.Name)
				break LOOP
			default:
				nextSeconds += m.Interval
				cpTime := m.startTime.Add(time.Duration(nextSeconds) * time.Second)
				sleepTime := cpTime.Sub(time.Now())
				if sleepTime > 0 {
					time.Sleep(sleepTime)
					metricValues := m.CollectMetrics(m.Num)
					if metricValues != nil {
						newCP := cache.Checkpoint{
							Time:         cpTime,
							Seconds:      nextSeconds,
							MetricValues: metricValues,
						}
						if !m.SkipSameMerics || !newCP.HasSameMetricValues(lastCp) {
							m.checkpoints = append(m.checkpoints, &newCP)
							//fmt.Printf("append checkpoint with metric values: %v\n", metricValues)
						}
						lastCp = &newCP
					}
					if m.StopTrigger != nil && m.StopTrigger(m) {
						m.Stop()
						fmt.Printf("Monitor[%s] is exiting since stop trigger works\n", m.Name)
						break LOOP
					}
				}
			}
		}
		fmt.Printf("Monitor[%s] exited\n", m.Name)
		m.wg.Done()
		// m.Wg2.Done()
	}(m)
	fmt.Printf("Monitor[%s] started\n", m.Name)
}

func (m *Monitor) Stop() {
	m.stopTime = time.Now()
	go func() {
		m.stopChan <- true
	}()
}

func (m *Monitor) WaitForStopped() {
	m.wg.Wait()
}

func WaitUtilAllMetricsAreCleanedUp(assigned *sync.WaitGroup, collectMetrics func(id int) []int, id int) {
	target := fmt.Sprintf(" %d", id)
	initMonitor := &Monitor{
		Name:           "clean-up-monitor" + target,
		Num:            id,
		Interval:       1,
		CollectMetrics: collectMetrics,
		StopTrigger: func(m *Monitor) bool {
			metricValues := m.GetLastCheckPoint()
			for _, v := range metricValues.MetricValues {
				if v != 0 {
					return false
				}
			}
			return true
		},
	}
	initMonitor.SetWG(assigned)
	initMonitor.Start()
	// initMonitor.WaitForStopped()
	fmt.Printf("Moniter-%s related pods are cleaned up\n", target)
}

func (m *Monitor) SetWG(assigned *sync.WaitGroup) {
	m.wg = assigned
}
