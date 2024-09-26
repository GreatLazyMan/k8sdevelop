package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/creasty/defaults"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	addr              = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	uniformDomain     = flag.Float64("uniform.domain", 0.0002, "The domain for the uniform distribution.")
	normDomain        = flag.Float64("normal.domain", 0.0002, "The domain for the normal distribution.")
	normMean          = flag.Float64("normal.mean", 0.00001, "The mean for the normal distribution.")
	oscillationPeriod = flag.Duration("oscillation-period", 10*time.Minute, "The duration of the rate oscillation period.")
)

var (
	// Objectives 表示quantile的误差范围
	// 假设某个0.5-quantile的值为120，由于设置的误差为0.05，所以120代表的真实quantile是(0.45, 0.55)范围内的某个值。
	// 即小于 120 的数据在整个数据中占比 45% ～ 55%
	rpcDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "rpc_durations_seconds",
			Help:       "RPC latency distributions.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"service", "error_code"},
	)
	rpcDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "rpc_durations_histogram_seconds",
		Help: "RPC latency distributions.",
		// LinearBuckets用于创建等差数列
		// ExponentialBucket用于创建等比数列
		Buckets: prometheus.LinearBuckets(5, 5, 20),
	})

	tempSummary = prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       "pond_temperature_celsius",
		Help:       "The temperature of the frog pond.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
)

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
	myGague = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "my_example_gauge_data",
		Help:        "my example gauge data",
		ConstLabels: map[string]string{"error": ""},
	})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(rpcDurations)
	prometheus.MustRegister(tempSummary)
	prometheus.MustRegister(rpcDurationsHistogram)
	prometheus.MustRegister(newFooCollector())
	// Add Go module build info.
	prometheus.MustRegister(collectors.NewBuildInfoCollector())
}

func recordMetrics() {
	go func() {
		for {
			opsProcessed.Inc()
			myGague.Add(11)
			time.Sleep(2 * time.Second)
		}
	}()
}

type Config struct {
	Loggie Loggie `yaml:"loggie"`
}

type Loggie struct {
	JSONEngine string `yaml:"jsonEngine,omitempty" default:"jsoniter" validate:"oneof=jsoniter sonic std go-json"`
}

func main() {
	flag.Parse()
	config := &Config{}
	defaults.Set(config)
	fmt.Println(config)

	start := time.Now()

	recordMetrics()

	oscillationFactor := func() float64 {
		return 2 + math.Sin(math.Sin(2*math.Pi*float64(time.Since(start))/float64(*oscillationPeriod)))
	}

	go func() {
		i := 1
		for {
			time.Sleep(time.Duration(75*oscillationFactor()) * time.Millisecond)
			if (i * 3) > 100 {
				break
			}
			tempSummary.Observe(11)
			rpcDurations.WithLabelValues("normal", "400").Observe(float64((i * 3) % 100))
			rpcDurationsHistogram.Observe(float64((i * 3) % 100))
			fmt.Println(float64((i*3)%100), " i=", i)
			i++
		}
	}()

	go func() {
		for {
			v := rand.ExpFloat64() / 1e6
			rpcDurations.WithLabelValues("exponential", "303").Observe(v)
			time.Sleep(time.Duration(50*oscillationFactor()) * time.Millisecond)
		}
	}()

	workerDB := NewClusterManager("db")
	workerCA := NewClusterManager("ca")
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(workerDB)
	reg.MustRegister(workerCA)
	// prometheus.Gatherers用来定义一个采集数据的收集器集合,可以merge多个不同的采集数据到一个结果集合
	// 这里我们传递了缺省的DefaultGatherer,所以他在输出中也会包含go运行时指标信息
	// 同时包含reg是我们之前生成的一个注册对象,用来自定义采集数据
	// 如果注释掉了prometheus.DefaultGatherername只会采集我们自己定义的指标
	gatherers := prometheus.Gatherers{
		//prometheus.DefaultGatherer,
		reg,
		prometheus.DefaultGatherer,
	}
	// promhttp.HandlerFor()函数传递之前的Gatherers对象,并返回一个httpHandler对象
	// 这个httpHandler对象可以调用其自身的ServHTTP函数来接手http请求并返回响应
	// 其中promhttp.HandlerOpts定义了采集过程中如果发生错误时,继续采集其他的数据。
	h := promhttp.HandlerFor(gatherers,
		promhttp.HandlerOpts{
			ErrorLog:      &log.Logger{Level: log.ErrorLevel},
			ErrorHandling: promhttp.ContinueOnError,
		})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	})
	// Expose the registered metrics via HTTP.
	//http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
