package admin

// ModelStats 模型统计信息
type ModelStats struct {
	ActiveModels    int               `json:"active_models"`
	TotalRequests   int               `json:"total_requests"`
	SuccessRequests int               `json:"success_requests"`
	FailedRequests  int               `json:"failed_requests"`
	ModelList       []ModelDetailInfo `json:"model_list"`
}

// ModelDetailInfo 模型详细信息
type ModelDetailInfo struct {
	ModelName       string  `json:"model_name"`
	TotalRequests   int     `json:"total_requests"`
	SuccessRequests int     `json:"success_requests"`
	FailedRequests  int     `json:"failed_requests"`
	SuccessRate     float64 `json:"success_rate"`
}
