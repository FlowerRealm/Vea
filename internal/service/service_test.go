package service

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"testing"
)

func TestShouldRetryHTTP11(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "EOF",
			err:  io.EOF,
			want: true,
		},
		{
			name: "unexpected EOF",
			err:  io.ErrUnexpectedEOF,
			want: true,
		},
		{
			name: "http2 protocol",
			err:  errors.New("http2: stream closed"),
			want: true,
		},
		{
			name: "protocol error",
			err:  errors.New("server protocol error"),
			want: true,
		},
		{
			name: "url error wrapping EOF",
			err:  &url.Error{Err: io.EOF},
			want: true,
		},
		{
			name: "url error wrapping http2 message",
			err:  &url.Error{Err: errors.New("HTTP2: PROTOCOL")},
			want: true,
		},
		{
			name: "op error wrapping EOF",
			err:  &net.OpError{Err: io.ErrUnexpectedEOF},
			want: true,
		},
		{
			name: "nested op/url/http2",
			err: &net.OpError{Err: &url.Error{Err: errors.New(
				"http2: unexpected stream error",
			)}},
			want: true,
		},
		{
			name: "error message contains EOF",
			err:  errors.New("server response EOF while reading body"),
			want: true,
		},
		{
			name: "nested error message contains EOF",
			err: &net.OpError{Err: &url.Error{Err: errors.New(
				"proxy tunnel failure: unexpected EOF",
			)}},
			want: true,
		},
		{
			name: "tls record header error",
			err:  &tls.RecordHeaderError{Msg: "first record does not look like TLS"},
			want: true,
		},
		{
			name: "non retryable",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "url error non retryable",
			err:  &url.Error{Err: errors.New("lookup failed")},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldRetryHTTP11(tc.err); got != tc.want {
				t.Fatalf("shouldRetryHTTP11(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
