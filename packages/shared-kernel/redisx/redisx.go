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

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"
)

type Client struct {
	addr        string
	dialTimeout time.Duration
	rwTimeout   time.Duration
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if strings.TrimSpace(key) == "" {
		return 0, nil
	}
	replies, err := c.Pipeline(ctx, []string{"INCR", key})
	if err != nil {
		return 0, err
	}
	if len(replies) != 1 {
		return 0, errors.New("redis: unexpected reply count")
	}
	n, ok := replies[0].(int64)
	if !ok {
		return 0, errors.New("redis: expected integer reply")
	}
	return n, nil
}

func (c *Client) TTL(ctx context.Context, key string) (ttl time.Duration, found bool, err error) {
	if strings.TrimSpace(key) == "" {
		return 0, false, nil
	}
	replies, err := c.Pipeline(ctx, []string{"TTL", key})
	if err != nil {
		return 0, false, err
	}
	if len(replies) != 1 {
		return 0, false, errors.New("redis: unexpected reply count")
	}
	n, ok := replies[0].(int64)
	if !ok {
		return 0, false, errors.New("redis: expected integer reply")
	}
	if n == -2 {
		return 0, false, nil
	}
	if n < 0 {
		return 0, true, nil
	}
	return time.Duration(n) * time.Second, true, nil
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

func (c *Client) Get(ctx context.Context, key string) (val string, found bool, err error) {
	if strings.TrimSpace(key) == "" {
		return "", false, nil
	}
	replies, err := c.Pipeline(ctx, []string{"GET", key})
	if err != nil {
		return "", false, err
	}
	if len(replies) != 1 {
		return "", false, errors.New("redis: unexpected reply count")
	}
	s, ok := replies[0].(string)
	if !ok {
		return "", false, errors.New("redis: expected bulk string reply")
	}
	if s == "" {
		// Our RESP decoder returns empty string for nil bulk replies ($-1).
		return "", false, nil
	}
	return s, true, nil
}

func (c *Client) SetEX(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	secs := int64(ttl / time.Second)
	if secs <= 0 {
		secs = 1
	}
	_, err := c.Pipeline(ctx, []string{"SET", key, string(value), "EX", strconv.FormatInt(secs, 10)})
	return err
}
