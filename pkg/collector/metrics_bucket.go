package collector

import (
	"encoding/json"
	"sync"

	"github.com/DesistDaydream/prometheus-instrumenting/pkg/scraper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	// check interface
	_ scraper.CommonScraper = ScrapeBuckets{}

	// 全部桶的总数
	bucketTotalCount = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_total_count"),
		"全部桶的总数",
		[]string{}, nil,
	)
	// 桶的总对象数
	numObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_num_objects"),
		"桶的总对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 通的总数据量
	size = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_size"),
		"桶的总数据量",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的本地总对象数
	localAllocatedObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_local_allocated_objects"),
		"桶的本地总对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的本地总数据量
	localAllocatedSize = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_local_allocated_size"),
		"桶的本地总数据量,Bytes",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的转磁带对象数
	externalTapeObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_external_tape_objects"),
		"桶的转磁带对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的转磁带数据量
	externalTapeSize = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_external_tape_size"),
		"桶的转磁带数据量",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的转光对象数
	externalGlacierObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_external_glacier_objects"),
		"桶的转光对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的转光数据量
	externalGlacierSize = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_external_glacier_size"),
		"桶的转光数据量",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的取回光对象数
	restoreGlacierObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_restore_glacier_objects"),
		"桶的取回光对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的取回光数据量
	restoreGlacierSize = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_restore_glacier_size"),
		"桶的取回光数据量",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的取回磁带对象数
	restoreTapeObjects = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_restore_tape_objects"),
		"桶的取回磁带对象数",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)
	// 桶的取回磁带数据量
	restoreTapeSize = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "bucket_restore_tape_size"),
		"桶的取回磁带数据量",
		[]string{"e37_bid", "e37_bucket_name", "e37_uid"}, nil,
	)

	// SizeKb
	// SizeKbUtilized
	// SizeUtilized
	// SizeKbActual
	// SizeActual
)

// ScrapeBuckets 是将要实现 Scraper 接口的一个 Metric 结构体
type ScrapeBuckets struct{}

// Name 指定自己定义的 抓取器 的名字，与 Metric 的名字不是一个概念，但是一般保持一致
// 该方法用于为 ScrapeBuckets 结构体实现 Scraper 接口
func (ScrapeBuckets) Name() string {
	return "buckets_info"
}

// Help 指定自己定义的 抓取器 的帮助信息，这里的 Help 的内容将会作为命令行标志的帮助信息。与 Metric 的 Help 不是一个概念。
// 该方法用于为 ScrapeBuckets 结构体实现 Scraper 接口
func (ScrapeBuckets) Help() string {
	return "E37 Buckets Info"
}

// Scrape 从客户端采集数据，并将其作为 Metric 通过 channel(通道) 发送。主要就是采集 E37 集群信息的具体行为。
// 该方法用于为 ScrapeBuckets 结构体实现 Scraper 接口
func (ScrapeBuckets) Scrape(client scraper.CommonClient, ch chan<- prometheus.Metric) (err error) {
	// 声明需要绑定的 响应体 与 结构体
	var (
		bucketsList bucketsList
	)

	url := "/api/rgw/bucket"
	respBody, err := client.Request("GET", url, nil)
	if err != nil {
		return err
	}

	err = json.Unmarshal(respBody, &bucketsList)
	if err != nil {
		return err
	}

	logrus.Debugf("所有桶列表：%v", bucketsList)

	// 全部桶的总数
	ch <- prometheus.MustNewConstMetric(bucketTotalCount, prometheus.GaugeValue, float64(len(bucketsList)))

	var wg sync.WaitGroup
	defer wg.Wait()

	// 用来控制并发数量
	concurrenceControl := make(chan bool, 10)

	for _, bucketName := range bucketsList {
		concurrenceControl <- true
		wg.Add(1)
		bucketUrl := "/api/rgw/bucket/" + bucketName
		bucketMethod := "GET"
		go func(bucketUrl string) {
			defer wg.Done()
			var bucketData bucket
			respBodyBucket, err := client.Request(bucketMethod, bucketUrl, nil)
			if err != nil {
				logrus.Errorf("获取 %v 桶数据失败，原因:%v", bucketUrl, err)
				<-concurrenceControl
				return
			}
			err = json.Unmarshal(respBodyBucket, &bucketData)
			if err != nil {
				logrus.Errorf("解析 %v 桶数据失败，原因:%v", bucketUrl, err)
				<-concurrenceControl
				return
			}
			// 桶的总对象数
			ch <- prometheus.MustNewConstMetric(numObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.NumObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 通的总数据量
			ch <- prometheus.MustNewConstMetric(size, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.Size),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的本地总对象数
			ch <- prometheus.MustNewConstMetric(localAllocatedObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.LocalAllocatedObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的本地总数据量
			ch <- prometheus.MustNewConstMetric(localAllocatedSize, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.LocalAllocatedSize),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的转磁带对象数
			ch <- prometheus.MustNewConstMetric(externalTapeObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.ExternalTapeObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的转磁带数据量
			ch <- prometheus.MustNewConstMetric(externalTapeSize, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.ExternalTapeSize),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的转光对象数
			ch <- prometheus.MustNewConstMetric(externalGlacierObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.ExternalGlacierObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的转光数据量
			ch <- prometheus.MustNewConstMetric(externalGlacierSize, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.ExternalGlacierSize),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的取回光对象数
			ch <- prometheus.MustNewConstMetric(restoreGlacierObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.RestoreGlacierObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的取回光数据量
			ch <- prometheus.MustNewConstMetric(restoreGlacierSize, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.RestoreGlacierSize),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的取回磁带对象数
			ch <- prometheus.MustNewConstMetric(restoreTapeObjects, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.RestoreTapeObjects),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			// 桶的取回磁带数据量
			ch <- prometheus.MustNewConstMetric(restoreTapeSize, prometheus.GaugeValue, float64(bucketData.Usage.RgwMain.RestoreTapeSize),
				bucketData.Bid,
				bucketData.Bucket,
				bucketData.Owner,
			)
			<-concurrenceControl
		}(bucketUrl)

		// logrus.Debugf("桶计数：%v\n", index)
		// if index > 200 {
		// 	break
		// }
	}

	return nil
}

type bucketsList []string

type bucket struct {
	ExplicitPlacement explicitPlacement `json:"explicit_placement"`
	Usage             usage             `json:"usage"`
	Bid               string            `json:"bid"`
	Bucket            string            `json:"bucket"`
	Owner             string            `json:"owner"`
	IndexType         string            `json:"index_type"`
	Mtime             string            `json:"mtime"`
	Marker            string            `json:"marker"`
	Zonegroup         string            `json:"zonegroup"`
	PlacementRule     string            `json:"placement_rule"`
	ID                string            `json:"id"`
	Tenant            string            `json:"tenant"`
	BucketQuota       bucketQuota       `json:"bucket_quota"`
}
type explicitPlacement struct {
	DataPool      string `json:"data_pool"`
	IndexPool     string `json:"index_pool"`
	DataExtraPool string `json:"data_extra_pool"`
}
type rgwMultimeta struct {
	ExternalGlacierSize    int64 `json:"external_glacier_size"`
	LocalAllocatedObjects  int64 `json:"local_allocated_objects"`
	RestoreTapeSize        int64 `json:"restore_tape_size"`
	SizeKb                 int64 `json:"size_kb"`
	ExternalGlacierObjects int64 `json:"external_glacier_objects"`
	ExternalTapeSize       int64 `json:"external_tape_size"`
	RestoreTapeObjects     int64 `json:"restore_tape_objects"`
	LocalAllocatedSize     int64 `json:"local_allocated_size"`
	NumObjects             int64 `json:"num_objects"`
	ExternalTapeObjects    int64 `json:"external_tape_objects"`
	SizeKbUtilized         int64 `json:"size_kb_utilized"`
	SizeUtilized           int64 `json:"size_utilized"`
	RestoreGlacierObjects  int64 `json:"restore_glacier_objects"`
	SizeKbActual           int64 `json:"size_kb_actual"`
	SizeActual             int64 `json:"size_actual"`
	RestoreGlacierSize     int64 `json:"restore_glacier_size"`
	Size                   int64 `json:"size"`
}
type rgwMain struct {
	ExternalGlacierSize    int64 `json:"external_glacier_size"`
	LocalAllocatedObjects  int64 `json:"local_allocated_objects"`
	RestoreTapeSize        int64 `json:"restore_tape_size"`
	SizeKb                 int64 `json:"size_kb"`
	ExternalGlacierObjects int64 `json:"external_glacier_objects"`
	ExternalTapeSize       int64 `json:"external_tape_size"`
	RestoreTapeObjects     int64 `json:"restore_tape_objects"`
	LocalAllocatedSize     int64 `json:"local_allocated_size"`
	NumObjects             int64 `json:"num_objects"`
	ExternalTapeObjects    int64 `json:"external_tape_objects"`
	SizeKbUtilized         int64 `json:"size_kb_utilized"`
	SizeUtilized           int64 `json:"size_utilized"`
	RestoreGlacierObjects  int64 `json:"restore_glacier_objects"`
	SizeKbActual           int64 `json:"size_kb_actual"`
	SizeActual             int64 `json:"size_actual"`
	RestoreGlacierSize     int64 `json:"restore_glacier_size"`
	Size                   int64 `json:"size"`
}
type usage struct {
	RgwMultimeta rgwMultimeta `json:"rgw.multimeta"`
	RgwMain      rgwMain      `json:"rgw.main"`
}
type bucketQuota struct {
	MaxObjects int64 `json:"max_objects"`
	Enabled    bool  `json:"enabled"`
	MaxSizeKb  int64 `json:"max_size_kb"`
	MaxSize    int64 `json:"max_size"`
	CheckOnRaw bool  `json:"check_on_raw"`
}
