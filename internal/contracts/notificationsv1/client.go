package notificationsv1

import (
	"context"
	"net/http"
	"time"

	"github.com/skelbigo/FinanceTracker/internal/grpcx"
)

type Client struct {
	addr string
	hc   *http.Client
}

func NewClient(addr string) *Client {
	return &Client{
		addr: addr,
		hc:   grpcx.NewH2CClient(),
	}
}

func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	if hc != nil {
		c.hc = hc
	}
	return c
}

func (c *Client) CreateNotification(ctx context.Context, req *CreateNotificationRequest) (*CreateNotificationResponse, error) {
	if req == nil {
		req = &CreateNotificationRequest{}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	in, err := req.Marshal()
	if err != nil {
		return nil, err
	}
	_, out, err := grpcx.InvokeUnary(ctx, c.hc, c.addr, FullMethodCreateNotification, in)
	if err != nil {
		return nil, err
	}
	var resp CreateNotificationResponse
	if err := resp.Unmarshal(out); err != nil {
		return nil, err
	}
	return &resp, nil
}
