package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type analyticsTopRowVM struct {
	Name     string
	Spent    string
	Percent  string
	SharePct float64
}

type analyticsChartVM struct {
	Labels   []string  `json:"labels"`
	Income   []float64 `json:"income"`
	Expense  []float64 `json:"expense"`
	Net      []float64 `json:"net"`
	Currency string    `json:"currency"`
}

type analyticsVM struct {
	Period           string
	From             string
	To               string
	GranularityInput string
	Granularity      string
	Currency         string
	Income           string
	Expense          string
	Net              string
	Top              []analyticsTopRowVM
	ChartJSON        string
}

func (h *Handlers) GetAnalyticsPage(c *gin.Context) {
	if h.Analytics == nil {
		c.String(http.StatusInternalServerError, "analytics service is not configured")
		return
	}

	wsIDRaw, ok := c.Get(workspaces.CtxWorkspaceIDKey)
	wsIDStr, _ := wsIDRaw.(string)
	if !ok || wsIDStr == "" {
		c.Redirect(http.StatusSeeOther, "/app?flash=Pick+a+workspace")
		return
	}
	wsUUID, err := uuid.Parse(wsIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/app?flash=Bad+workspace")
		return
	}

	period := c.Query("period")
	if period == "" {
		period = "this_month"
	}

	fromQ := c.Query("from")
	toQ := c.Query("to")
	g := c.Query("groupBy")
	if g == "" {
		g = c.Query("g")
	}
	if g == "" {
		g = "auto"
	}

	cur := c.Query("cur")
	if cur == "" {
		cur = c.Query("currency")
	}
	if cur == "" {
		if wRaw, ok := c.Get("workspace"); ok {
			if w, ok := wRaw.(*workspaces.Workspace); ok && w != nil && w.DefaultCurrency != "" {
				cur = w.DefaultCurrency
			}
			if w, ok := wRaw.(workspaces.Workspace); ok && w.DefaultCurrency != "" {
				cur = w.DefaultCurrency
			}
		}
	}
	if cur == "" {
		cur = "UAH"
	}

	now := time.Now()
	loc := now.Location()
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)

	var from, to time.Time
	switch period {
	case "last_month":
		firstThisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		lastMonthEnd := firstThisMonth.AddDate(0, 0, -1)
		from = time.Date(lastMonthEnd.Year(), lastMonthEnd.Month(), 1, 0, 0, 0, 0, loc)
		to = lastMonthEnd
	case "custom":
		f, ferr := time.ParseInLocation("2006-01-02", fromQ, loc)
		t, terr := time.ParseInLocation("2006-01-02", toQ, loc)
		if ferr != nil || terr != nil {
			from = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
			to = today
			period = "this_month"
		} else {
			from = time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
			to = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			if to.Before(from) {
				from, to = to, from
			}
		}
	default:
		from = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		to = today
		period = "this_month"
	}

	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	groupBy := g
	if groupBy == "auto" {
		days := int(to.Sub(from).Hours()/24) + 1
		switch {
		case days <= 45:
			groupBy = "day"
		case days <= 180:
			groupBy = "week"
		default:
			groupBy = "month"
		}
	}

	resp, aerr := h.Analytics.Analytics(c.Request.Context(), wsUUID, fromStr, toStr, cur, groupBy, 10)
	if aerr != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("analytics error: %v", aerr))
		return
	}

	chart := analyticsChartVM{Currency: cur}
	chart.Labels = make([]string, 0, len(resp.Cashflow))
	chart.Income = make([]float64, 0, len(resp.Cashflow))
	chart.Expense = make([]float64, 0, len(resp.Cashflow))
	chart.Net = make([]float64, 0, len(resp.Cashflow))
	for _, p := range resp.Cashflow {
		chart.Labels = append(chart.Labels, p.Bucket)
		chart.Income = append(chart.Income, float64(p.Income)/100.0)
		chart.Expense = append(chart.Expense, float64(p.Expense)/100.0)
		chart.Net = append(chart.Net, float64(p.Net)/100.0)
	}
	chartBytes, _ := json.Marshal(chart)

	vm := analyticsVM{
		Period:           period,
		From:             fromStr,
		To:               toStr,
		GranularityInput: g,
		Granularity:      groupBy,
		Currency:         cur,
		Income:           formatMinor(resp.Totals.Income),
		Expense:          formatMinor(resp.Totals.Expense),
		Net:              formatMinor(resp.Totals.Net),
		ChartJSON:        string(chartBytes),
	}

	vm.Top = make([]analyticsTopRowVM, 0, len(resp.TopCategories))
	for _, r := range resp.TopCategories {
		sharePct := r.Share * 100
		if sharePct < 0 {
			sharePct = 0
		}
		if sharePct > 100 {
			sharePct = 100
		}
		vm.Top = append(vm.Top, analyticsTopRowVM{
			Name:     r.Name,
			Spent:    formatMinor(r.Total),
			Percent:  fmt.Sprintf("%.0f%%", r.Share*100),
			SharePct: sharePct,
		})
	}

	h.render(c, "app/analytics.html", gin.H{
		"Title":     "Analytics",
		"BodyClass": "app-dark",
		"MainClass": "dash-main",
		"VM":        vm,
	})
}
