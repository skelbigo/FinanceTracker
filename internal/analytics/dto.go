package analytics

type SummaryResponse struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Currency     string `json:"currency"`
	IncomeTotal  int64  `json:"income_total"`
	ExpenseTotal int64  `json:"expense_total"`
	Net          int64  `json:"net"`
}

type ByCategoryItem struct {
	CategoryID *string `json:"category_id"`
	Name       string  `json:"name"`
	Total      int64   `json:"total"`
	Count      int64   `json:"count,omitempty"`
	Share      float64 `json:"share"`
}

type ByCategoryResponse struct {
	From     string           `json:"from"`
	To       string           `json:"to"`
	Currency string           `json:"currency"`
	Type     string           `json:"type"`
	Total    int64            `json:"total"`
	Items    []ByCategoryItem `json:"items"`
}

type TimeseriesPoint struct {
	Period string `json:"period"`
	Total  int64  `json:"total"`
}

type TimeseriesResponse struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Currency string            `json:"currency"`
	Bucket   string            `json:"bucket"`
	Type     string            `json:"type"`
	Points   []TimeseriesPoint `json:"points"`
}
