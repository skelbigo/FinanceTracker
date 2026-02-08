package notifications

import (
	"context"
	"log"
	"time"
)

type EmailWorker struct {
	repo         *Repo
	sender       EmailSender
	publicURL    string
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	logger       *log.Logger
}

func NewEmailWorker(repo *Repo, sender EmailSender, publicURL string, pollInterval time.Duration, batchSize, maxAttempts int, logger *log.Logger) *EmailWorker {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}
	if logger == nil {
		logger = log.Default()
	}
	return &EmailWorker{
		repo:         repo,
		sender:       sender,
		publicURL:    publicURL,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		maxAttempts:  maxAttempts,
		logger:       logger,
	}
}

func (w *EmailWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.logger.Printf("Email worker started: interval=%s batch=%d maxAttempts=%d", w.pollInterval, w.batchSize, w.maxAttempts)

	for {
		select {
		case <-ctx.Done():
			w.logger.Printf("Email worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *EmailWorker) tick(ctx context.Context) {
	if w.sender == nil || !w.sender.Enabled() {
		return
	}

	jobs, err := w.repo.ClaimPendingEmailDeliveries(ctx, w.batchSize, w.maxAttempts)
	if err != nil {
		w.logger.Printf("email worker: claim pending: %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	for _, j := range jobs {
		w.processJob(ctx, j)
	}
}

func (w *EmailWorker) processJob(ctx context.Context, job EmailDeliveryJob) {
	if job.ToEmail == "" {
		errTxt := "missing recipient email"
		_ = w.repo.UpdateDeliveryByID(ctx, job.DeliveryID, StatusFailed, &errTxt, nil)
		return
	}

	subject, html, err := renderNotificationEmail(job.Notif, w.publicURL)
	if err != nil {
		errTxt := err.Error()
		_ = w.repo.UpdateDeliveryByID(ctx, job.DeliveryID, StatusFailed, &errTxt, nil)
		return
	}

	err = w.sender.Send(job.ToEmail, subject, html)
	if err != nil {
		errTxt := err.Error()
		_ = w.repo.UpdateDeliveryByID(ctx, job.DeliveryID, StatusFailed, &errTxt, nil)
		w.logger.Printf("email send failed: delivery=%s notif=%s to=%s err=%v", job.DeliveryID, job.Notif.ID, job.ToEmail, err)
		return
	}

	now := time.Now()
	_ = w.repo.UpdateDeliveryByID(ctx, job.DeliveryID, StatusSent, nil, &now)
}
