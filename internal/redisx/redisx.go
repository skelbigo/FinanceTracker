package redisx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/skelbigo/FinanceTracker/internal/config"
)

type Client struct {
	addr        string
	dialTimeout time.Duration
	rwTimeout   time.Duration
}

func NewClient(cfg config.Config) *Client {
	if !cfg.RedisEnabled {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	return &Client{addr: addr, dialTimeout: 2 * time.Second, rwTimeout: 2 * time.Second}
}

func (c *Client) dial(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	d := net.Dialer{Timeout: c.dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(c.rwTimeout))
	return conn, bufio.NewReader(conn), nil
}

func (c *Client) Pipeline(ctx context.Context, cmds ...[]string) ([]any, error) {
	conn, r, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	for _, cmd := range cmds {
		if err := writeCommand(conn, cmd); err != nil {
			return nil, err
		}
	}

	out := make([]any, 0, len(cmds))
	for range cmds {
		v, err := readResp(r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) SAdd(ctx context.Context, key string, members ...string) error {
	if key == "" || len(members) == 0 {
		return nil
	}
	args := append([]string{"SADD", key}, members...)
	_, err := c.Pipeline(ctx, args)
	return err
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if key == "" {
		return nil
	}
	secs := int64(ttl / time.Second)
	if secs <= 0 {
		secs = 1
	}
	_, err := c.Pipeline(ctx, []string{"EXPIRE", key, strconv.FormatInt(secs, 10)})
	return err
}

func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	if key == "" {
		return nil, nil
	}
	replies, err := c.Pipeline(ctx, []string{"SMEMBERS", key})
	if err != nil {
		return nil, err
	}
	if len(replies) != 1 {
		return nil, errors.New("redis: unexpected reply count")
	}
	arr, ok := replies[0].([]any)
	if !ok {
		return nil, errors.New("redis: expected array reply")
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	filtered := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) != "" {
			filtered = append(filtered, k)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	args := append([]string{"DEL"}, filtered...)
	_, err := c.Pipeline(ctx, args)
	return err
}
