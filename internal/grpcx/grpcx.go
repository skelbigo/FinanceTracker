package grpcx

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/http2"
)

const ContentType = "application/grpc"

func WriteUnaryMessage(w io.Writer, msg []byte) error {
	var hdr [5]byte
	hdr[0] = 0
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(msg)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(msg)
	return err
}

func ReadUnaryMessage(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	var hdr [5]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return nil, err
	}
	if hdr[0] != 0 {
		return nil, fmt.Errorf("grpcx: compression not supported (flag=%d)", hdr[0])
	}
	n := binary.BigEndian.Uint32(hdr[1:5])
	if n == 0 {
		return []byte{}, nil
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(br, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func statusFromResponse(resp *http.Response) (code string, msg string) {
	code = resp.Trailer.Get("Grpc-Status")
	msg = resp.Trailer.Get("Grpc-Message")

	if code == "" {
		code = resp.Header.Get("Grpc-Status")
	}
	if msg == "" {
		msg = resp.Header.Get("Grpc-Message")
	}
	return
}

func EnsureTrailers(w http.ResponseWriter) {
	w.Header().Set("Trailer", "Grpc-Status, Grpc-Message")
	w.Header().Set("Content-Type", ContentType)
}

func WriteOKTrailers(w http.ResponseWriter) {
	w.Header().Set("Grpc-Status", "0")
}

func WriteErrorTrailers(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Grpc-Status", fmt.Sprintf("%d", code))
	w.Header().Set("Grpc-Message", message)
}

func NewH2CClient() *http.Client {
	t := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
	return &http.Client{Transport: t}
}

func InvokeUnary(ctx context.Context, client *http.Client, addr string, fullMethod string, reqBytes []byte) (*http.Response, []byte, error) {
	if client == nil {
		return nil, nil, errors.New("grpcx: nil http client")
	}
	if !strings.HasPrefix(fullMethod, "/") {
		fullMethod = "/" + fullMethod
	}

	url := "http://" + addr + fullMethod
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", ContentType)
	httpReq.Header.Set("TE", "trailers")

	pr, pw := io.Pipe()
	httpReq.Body = pr
	go func() {
		defer pw.Close()
		_ = WriteUnaryMessage(pw, reqBytes)
	}()

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBytes, err := ReadUnaryMessage(resp.Body)
	if err != nil {
		return resp, nil, err
	}

	code, gmsg := statusFromResponse(resp)
	if code != "" && code != "0" {
		if gmsg == "" {
			gmsg = "gRPC error"
		}
		return resp, respBytes, fmt.Errorf("grpcx: status=%s message=%s", code, gmsg)
	}

	return resp, respBytes, nil
}
