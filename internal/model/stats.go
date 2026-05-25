package model

// ChannelStats holds push statistics for a single channel within a time range.
type ChannelStats struct {
	Channel        string  `json:"channel"`
	Total          int     `json:"total"`
	SuccessCount   int     `json:"success_count"`
	FailedCount    int     `json:"failed_count"`
	SuccessPercent float64 `json:"success_percent"`
	FailedPercent  float64 `json:"failed_percent"`
}

// PagedResult is a generic paginated result container.
type PagedResult[T any] struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Data     []T `json:"data"`
}
