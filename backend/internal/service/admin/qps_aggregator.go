package admin

// QPSDataPoint QPS 数据点
type QPSDataPoint struct {
	Timestamp string  `json:"timestamp"`
	QPS       float64 `json:"qps"`
}
