package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

type EmailSender interface {
	Send(to string, subject string, htmlBody string) error
	Enabled() bool
}

type NoopEmailSender struct{}

func (NoopEmailSender) Send(to, subject, htmlBody string) error { return nil }
func (NoopEmailSender) Enabled() bool                           { return false }

type emailLayoutData struct {
	Title       string
	Body        template.HTML
	ButtonLabel string
	ButtonURL   string
	Footer      string
}

var baseEmailTpl = template.Must(template.New("base").Parse(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{ .Title }}</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica, Arial, sans-serif; background:#0B1220; color:#fff; padding:24px;">
  <div style="max-width:640px; margin:0 auto; background:rgba(255,255,255,0.06); border:1px solid rgba(255,255,255,0.10); border-radius:16px; overflow:hidden;">
    <div style="padding:20px 22px; background:rgba(32,242,200,0.10); border-bottom:1px solid rgba(255,255,255,0.10);">
      <div style="font-size:14px; letter-spacing:0.12em; text-transform:uppercase; color:rgba(255,255,255,0.75); font-weight:700;">Finance Tracker</div>
      <div style="font-size:22px; font-weight:800; margin-top:6px;">{{ .Title }}</div>
    </div>
    <div style="padding:18px 22px; font-size:15px; line-height:1.55; color:rgba(255,255,255,0.92);">
      {{ .Body }}
      {{ if .ButtonURL }}
      <div style="margin-top:18px;">
        <a href="{{ .ButtonURL }}" style="display:inline-block; padding:11px 14px; border-radius:12px; background:#20F2C8; color:#00110E; font-weight:800; text-decoration:none;">{{ if .ButtonLabel }}{{ .ButtonLabel }}{{ else }}Open app{{ end }}</a>
      </div>
      {{ end }}
    </div>
    <div style="padding:14px 22px; font-size:12px; color:rgba(255,255,255,0.65); border-top:1px solid rgba(255,255,255,0.10);">
      {{ if .Footer }}{{ .Footer }}{{ else }}Sent at {{ .Now }}{{ end }}
    </div>
  </div>
</body>
</html>`))

type baseEmailWrapper struct {
	emailLayoutData
	Now string
}

func renderEmail(title string, bodyHTML template.HTML, buttonLabel, buttonURL string) (string, error) {
	buf := new(bytes.Buffer)
	data := baseEmailWrapper{
		emailLayoutData: emailLayoutData{
			Title:       title,
			Body:        bodyHTML,
			ButtonLabel: buttonLabel,
			ButtonURL:   buttonURL,
		},
		Now: time.Now().Format(time.RFC1123),
	}
	if err := baseEmailTpl.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type overspendingEmailData struct {
	Currency    string
	Spent       string
	Limit       string
	OverBy      string
	PeriodStart string
	PeriodEnd   string
}

type newTransactionEmailData struct {
	Currency  string
	Amount    string
	TxType    string
	Date      string
	CreatedBy string
	Category  string
}

var overspendingBodyTpl = template.Must(template.New("overspending_body").Parse(`
<p style="margin:0 0 12px 0;">You have exceeded your budget.</p>
<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%; border-collapse:collapse;">
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Spent</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .Spent }}</td>
  </tr>
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Limit</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .Limit }}</td>
  </tr>
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Over by</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .OverBy }}</td>
  </tr>
</table>
{{ if and .PeriodStart .PeriodEnd }}
  <p style="margin:12px 0 0 0; color:rgba(255,255,255,0.75); font-size:13px;">Period: {{ .PeriodStart }} – {{ .PeriodEnd }}</p>
{{ end }}
`))

var newTransactionBodyTpl = template.Must(template.New("new_transaction_body").Parse(`
<p style="margin:0 0 12px 0;">A new {{ .TxType }} transaction was added.</p>
<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%; border-collapse:collapse;">
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Amount</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .Amount }}</td>
  </tr>
  {{ if .Category }}
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Category</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .Category }}</td>
  </tr>
  {{ end }}
  {{ if .Date }}
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Date</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .Date }}</td>
  </tr>
  {{ end }}
  {{ if .CreatedBy }}
  <tr>
    <td style="padding:8px 0; color:rgba(255,255,255,0.75);">Created by</td>
    <td style="padding:8px 0; text-align:right; font-weight:800;">{{ .CreatedBy }}</td>
  </tr>
  {{ end }}
</table>
`))

func renderOverspendingBody(data overspendingEmailData) (template.HTML, error) {
	buf := new(bytes.Buffer)
	if err := overspendingBodyTpl.Execute(buf, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func renderNewTransactionBody(data newTransactionEmailData) (template.HTML, error) {
	buf := new(bytes.Buffer)
	if err := newTransactionBodyTpl.Execute(buf, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func formatMoney(currency string, minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = int64(math.Abs(float64(minor)))
	}
	major := minor / 100
	frac := minor % 100
	cur := strings.TrimSpace(currency)
	if cur == "" {
		cur = ""
	} else {
		cur = cur + " "
	}
	return fmt.Sprintf("%s%s%d.%02d", sign, cur, major, frac)
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func shortDate(v any) string {
	s := asString(v)
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func renderNotificationEmail(n Notification, publicURL string) (subject string, html string, err error) {
	if strings.TrimSpace(publicURL) == "" {
		publicURL = ""
	}

	buttonLabel := "Open notifications"
	buttonURL := strings.TrimRight(publicURL, "/") + "/app/notifications"

	var body template.HTML
	switch n.Type {
	case TypeOverspending:
		cur := asString(n.Payload["currency"])
		spent, _ := asInt64(n.Payload["spent_minor"])
		limit, _ := asInt64(n.Payload["limit_minor"])
		over := spent - limit
		bd := overspendingEmailData{
			Currency:    cur,
			Spent:       formatMoney(cur, spent),
			Limit:       formatMoney(cur, limit),
			OverBy:      formatMoney(cur, over),
			PeriodStart: shortDate(n.Payload["period_start"]),
			PeriodEnd:   shortDate(n.Payload["period_end"]),
		}
		body, err = renderOverspendingBody(bd)
		if err != nil {
			return "", "", err
		}
		buttonLabel = "View budgets"
		buttonURL = strings.TrimRight(publicURL, "/") + "/app/budgets"
	case TypeNewTransaction:
		cur := asString(n.Payload["currency"])
		amt, _ := asInt64(n.Payload["amountMinor"])
		txType := asString(n.Payload["type"])
		if txType == "" {
			txType = "transaction"
		}
		bd := newTransactionEmailData{
			Currency:  cur,
			Amount:    formatMoney(cur, amt),
			TxType:    txType,
			Date:      shortDate(n.Payload["date"]),
			CreatedBy: asString(n.Payload["createdBy"]),
			Category:  asString(n.Payload["categoryId"]),
		}
		body, err = renderNewTransactionBody(bd)
		if err != nil {
			return "", "", err
		}
		buttonLabel = "View transactions"
		buttonURL = strings.TrimRight(publicURL, "/") + "/app/transactions"
	default:
		body = template.HTML(fmt.Sprintf("<p>%s</p>", template.HTMLEscapeString(n.Body)))
	}

	subject = n.Title
	full, err := renderEmail(n.Title, body, buttonLabel, buttonURL)
	if err != nil {
		return "", "", err
	}
	return subject, full, nil
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case float32:
		return int64(t), true
	case json.Number:
		i, err := t.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
