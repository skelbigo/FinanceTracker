package notifications

import (
	"bytes"
	"html/template"
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
