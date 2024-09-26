package main

import (
	"math/rand"

	"github.com/prometheus/client_golang/prometheus"
)

// 先定义结构体,这是一个集群的指标采集器
// 每个集群都有自己的Zone,代表集群的名称,另外两个是保存的采集的指标
type ClusterManager struct {
	Zone         string
	OOMCountDesc *prometheus.Desc
	RAMUsageDesc *prometheus.Desc
}

// 返回一个按照主机名作为键采集到的数据,两个返回值分别代表了OOM错误计数和RAM使用指标信息
func (c *ClusterManager) ReallyExpensiveAssessmentOfTheSystemState() (
	oomCountByHost map[string]int, ramUsageByHost map[string]float64,
) {
	oomCountByHost = map[string]int{
		"foo.example.org": int(rand.Int31n(1000)),
		"bar.example.org": int(rand.Int31n(1000)),
	}
	ramUsageByHost = map[string]float64{
		"foo.example.org": rand.Float64() * 100,
		"bar.example.org": rand.Float64() * 100,
	}
	return
}

// 实现Describe接口,传递指标描述符到channel
func (c *ClusterManager) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.OOMCountDesc
	ch <- c.RAMUsageDesc
}

// Collect函数将执行抓取函数并返回数据,返回的数据传递到channel中,并且传递的同时绑定原先的指标描述符
// 以及指标的类型(一个Counter和一个Gauge)
func (c *ClusterManager) Collect(ch chan<- prometheus.Metric) {
	oomCountByHost, ramUsageByHost := c.ReallyExpensiveAssessmentOfTheSystemState()
	for host, oomCount := range oomCountByHost {
		ch <- prometheus.MustNewConstMetric(
			c.OOMCountDesc,
			prometheus.CounterValue,
			float64(oomCount),
			host,
		)
	}
	for host, ramUsage := range ramUsageByHost {
		ch <- prometheus.MustNewConstMetric(
			c.RAMUsageDesc,
			prometheus.GaugeValue,
			ramUsage,
			host,
		)
	}
}

// 创建结构体及对应的指标信息,NewDesc参数第一个为指标的名称,第二个为帮助信息,显示在指标的上面作为注释
// 第三个是定义的label名称数组,第四个是定义的Labels
func NewClusterManager(zone string) *ClusterManager {
	return &ClusterManager{
		Zone: zone,
		OOMCountDesc: prometheus.NewDesc(
			"clustermanager_oom_crashes_total",
			"Number of OOM crashes.",
			[]string{"host"},
			prometheus.Labels{"zone": zone},
		),
		RAMUsageDesc: prometheus.NewDesc(
			"clustermanager_ram_usage_bytes",
			"RAM usage as reported to the cluster manager.",
			[]string{"host"},
			prometheus.Labels{"zone": zone},
		),
	}
}

//  Collector：收集应用或者其他系统的指标，然后将其转化为Prometheus可识别收集的指标。
//  Exporter：它会从Collector获取指标数据，并将其转成为Prometheus可读格式。

type fooCollector struct {
	fooMetric *prometheus.Desc
	barMetric *prometheus.Desc
}

func newFooCollector() *fooCollector {
	return &fooCollector{
		// # HELP foo_metric Shows whether a foo has occurred in our cluster
		// # TYPE foo_metric counter
		// foo_metric 1
		fooMetric: prometheus.NewDesc("foo_metric",
			"Shows whether a foo has occurred in our cluster",
			nil, nil,
		),
		// # HELP bar_metric Shows whether a bar has occurred in our cluster
		// # TYPE bar_metric counter
		// bar_metric 1
		barMetric: prometheus.NewDesc("bar_metric",
			"Shows whether a bar has occurred in our cluster",
			nil, nil,
		),
	}
}

func (collector *fooCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.fooMetric
	ch <- collector.barMetric
}

func (collector *fooCollector) Collect(ch chan<- prometheus.Metric) {
	var metricValue float64
	if 1 == 1 {
		metricValue = 1
	}
	ch <- prometheus.MustNewConstMetric(collector.fooMetric, prometheus.CounterValue, metricValue)
	ch <- prometheus.MustNewConstMetric(collector.barMetric, prometheus.CounterValue, metricValue)
}
