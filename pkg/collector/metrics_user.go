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
	_ scraper.CommonScraper = ScrapeUsers{}

	// 全部用户总数
	userTotalCount = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "user_total_count"),
		"全部用户总数",
		[]string{}, nil,
	)
	// 用户的总请求数
	userTotalOps = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "user_total_ops"),
		"用户的总请求数",
		[]string{"e37_uid"}, nil,
	)
	// 用户总的成功请求数
	userTotalSuccessfulOps = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "user_total_successful_ops"),
		"用户总的成功请求数",
		[]string{"e37_uid"}, nil,
	)
	// 用户总对象数
	userTotalEntries = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "user_total_entries"),
		"用户总对象数",
		[]string{"e37_uid"}, nil,
	)
	// 用户总数据量
	userTotalBytes = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "object_storage", "user_total_bytes"),
		"用户总数据量",
		[]string{"e37_uid"}, nil,
	)
)

// ScrapeUsers 是将要实现 Scraper 接口的一个 Metric 结构体
type ScrapeUsers struct{}

// Name 指定自己定义的 抓取器 的名字，与 Metric 的名字不是一个概念，但是一般保持一致
// 该方法用于为 ScrapeUsers 结构体实现 Scraper 接口
func (ScrapeUsers) Name() string {
	return "UsersList_info"
}

// Help 指定自己定义的 抓取器 的帮助信息，这里的 Help 的内容将会作为命令行标志的帮助信息。与 Metric 的 Help 不是一个概念。
// 该方法用于为 ScrapeUsers 结构体实现 Scraper 接口
func (ScrapeUsers) Help() string {
	return "E37 UsersList Info"
}

// Scrape 从客户端采集数据，并将其作为 Metric 通过 channel(通道) 发送。主要就是采集 E37 集群信息的具体行为。
// 该方法用于为 ScrapeUsers 结构体实现 Scraper 接口
func (ScrapeUsers) Scrape(client scraper.CommonClient, ch chan<- prometheus.Metric) (err error) {
	// 声明需要绑定的 响应体 与 结构体
	var (
		usersListData usersList
	)

	url := "/api/rgw/user"
	respBody, err := client.Request("GET", url, nil)
	if err != nil {
		return err
	}

	err = json.Unmarshal(respBody, &usersListData)
	if err != nil {
		return err
	}

	logrus.Debugf("所有用户列表：%v", usersListData.Keys)

	// 全部用户总数
	ch <- prometheus.MustNewConstMetric(userTotalCount, prometheus.GaugeValue, float64(usersListData.Count))

	var wg sync.WaitGroup
	defer wg.Wait()

	// 用来控制并发数量
	concurrenceyControl := make(chan bool, client.GetConcurrency())

	for _, userName := range usersListData.Keys {
		concurrenceyControl <- true
		wg.Add(1)
		userUrl := "/api/rgw/user/" + userName
		userMethod := "GET"
		go func(userUrl string) {
			defer wg.Done()
			var userData user
			respBodyUser, err := client.Request(userMethod, userUrl, nil)
			if err != nil {
				logrus.Errorf("获取 %v 用户数据失败，原因:%v", userUrl, err)
				<-concurrenceyControl
				return
			}
			err = json.Unmarshal(respBodyUser, &userData)
			if err != nil {
				logrus.Errorf("解析 %v 用户数据失败，原因:%v", userUrl, err)
				<-concurrenceyControl
				return
			}
			// 用户的总请求
			if len(userData.Summary) > 0 && len(userData.Keys) > 0 {
				ch <- prometheus.MustNewConstMetric(userTotalOps, prometheus.GaugeValue, float64(userData.Summary[0].Total.Ops),
					userData.Keys[0].User,
				)
				// 用户总的成功请求数
				ch <- prometheus.MustNewConstMetric(userTotalSuccessfulOps, prometheus.GaugeValue, float64(userData.Summary[0].Total.SuccessfulOps),
					userData.Keys[0].User,
				)
				// 用户总对象数
				ch <- prometheus.MustNewConstMetric(userTotalEntries, prometheus.GaugeValue, float64(userData.Summary[0].Total.TotalEntries),
					userData.Keys[0].User,
				)
				// 用户总数据量
				ch <- prometheus.MustNewConstMetric(userTotalBytes, prometheus.GaugeValue, float64(userData.Summary[0].Total.TotalBytes),
					userData.Keys[0].User,
				)
			}
			<-concurrenceyControl
		}(userUrl)

		// logrus.Debugf("用户计数:%v\n", index)
		// if index > 20 {
		// 	break
		// }
	}

	return nil
}

// 用户统计信息
type usersList struct {
	Keys    []string  `json:"keys"`
	Count   int64     `json:"count"`
	Entries []entries `json:"entries"`
	Summary []summary `json:"summary"`
}
type categories struct {
	Category      string `json:"category"`
	SuccessfulOps int64  `json:"successful_ops"`
	BytesReceived int64  `json:"bytes_received"`
	BytesSent     int64  `json:"bytes_sent"`
	Ops           int64  `json:"ops"`
}
type buckets struct {
	Owner      string       `json:"owner"`
	Epoch      int64        `json:"epoch"`
	Bucket     string       `json:"bucket"`
	Categories []categories `json:"categories"`
	Time       string       `json:"time"`
}
type entries struct {
	Buckets []buckets `json:"buckets"`
	User    string    `json:"user"`
}
type total struct {
	SuccessfulOps int64 `json:"successful_ops"`
	BytesReceived int64 `json:"bytes_received"`
	BytesSent     int64 `json:"bytes_sent"`
	Ops           int64 `json:"ops"`
}
type summary struct {
	Total      total        `json:"total"`
	User       string       `json:"user"`
	Categories []categories `json:"categories"`
}

// 单用户统计信息
type user struct {
	PlacementTags       []string        `json:"placement_tags"`
	Keys                []userKeys      `json:"keys"`
	Entries             []userEntries   `json:"entries"`
	TempURLKeys         []string        `json:"temp_url_keys"`
	Caps                []usercaps      `json:"caps"`
	DefaultStorageClass string          `json:"default_storage_class"`
	Suspended           int64           `json:"suspended"`
	OpMask              string          `json:"op_mask"`
	SwiftKeys           []string        `json:"swift_keys"`
	Subuser             []string        `json:"subuser"`
	UID                 string          `json:"uid"`
	DisplayName         string          `json:"display_name"`
	UserID              string          `json:"user_id"`
	Admin               string          `json:"admin"`
	DefaultPlacement    string          `json:"default_placement"`
	System              string          `json:"system"`
	Summary             []userSummary   `json:"summary"`
	MaxBuckets          int64           `json:"max_buckets"`
	MfaIds              []string        `json:"mfa_ids"`
	Tenant              string          `json:"tenant"`
	Type                string          `json:"type"`
	Email               string          `json:"email"`
	UserQuota           userUserQuota   `json:"user_quota"`
	BucketQuota         userBucketQuota `json:"bucket_quota"`
}
type userKeys struct {
	User      string `json:"user"`
	SecretKey string `json:"secret_key"`
	AccessKey string `json:"access_key"`
}
type usercaps struct {
	Type string `json:"type"`
	Perm string `json:"perm"`
}
type userCategories struct {
	Category      string `json:"category"`
	SuccessfulOps int64  `json:"successful_ops"`
	BytesReceived int64  `json:"bytes_received"`
	BytesSent     int64  `json:"bytes_sent"`
	Ops           int64  `json:"ops"`
}
type userBuckets struct {
	Owner      string           `json:"owner"`
	Epoch      int64            `json:"epoch"`
	Bucket     string           `json:"bucket"`
	Categories []userCategories `json:"categories"`
	Time       string           `json:"time"`
}
type userEntries struct {
	Buckets []userBuckets `json:"buckets"`
	User    string        `json:"user"`
}
type userTotal struct {
	Ops           int64 `json:"ops"`
	SuccessfulOps int64 `json:"successful_ops"`
	TotalEntries  int64 `json:"total_entries"`
	BytesSent     int64 `json:"bytes_sent"`
	TotalBytes    int64 `json:"total_bytes"`
	BytesReceived int64 `json:"bytes_received"`
}
type userSummary struct {
	User       string           `json:"user"`
	Categories []userCategories `json:"categories"`
	Total      userTotal        `json:"total"`
}
type userUserQuota struct {
	MaxObjects int64 `json:"max_objects"`
	Enabled    bool  `json:"enabled"`
	MaxSizeKb  int64 `json:"max_size_kb"`
	MaxSize    int64 `json:"max_size"`
	CheckOnRaw bool  `json:"check_on_raw"`
}
type userBucketQuota struct {
	MaxObjects int64 `json:"max_objects"`
	Enabled    bool  `json:"enabled"`
	MaxSizeKb  int64 `json:"max_size_kb"`
	MaxSize    int64 `json:"max_size"`
	CheckOnRaw bool  `json:"check_on_raw"`
}
