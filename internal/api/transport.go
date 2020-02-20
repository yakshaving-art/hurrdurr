package api

import (
	"net/http"
	"time"
)

func newBackoffTransport() *http.Client {
	return &http.Client{
		Transport: backoffTransport{
			r:               http.DefaultTransport,
			backoffLimit:    5,
			backoffDuration: time.Millisecond * 500,
		},
	}
}

type backoffTransport struct {
	r http.RoundTripper

	backoffLimit    int
	backoffDuration time.Duration
}

func (b backoffTransport) RoundTrip(req *http.Request) (r *http.Response, err error) {
	bk := &backoff{
		Limit:    b.backoffLimit,
		Duration: b.backoffDuration,
	}
	for bk.wait() {
		r, err = b.r.RoundTrip(req)
		if err != nil {
			break
		}

		switch r.StatusCode {
		case http.StatusTooManyRequests:
			continue
		default:
			break
		}
	}
	return r, err
}

type backoff struct {
	current int

	Limit    int
	Duration time.Duration
}

func (b *backoff) wait() bool {
	if b.current == b.Limit {
		return false
	}
	b.current++

	time.Sleep(time.Duration(b.current) * b.Duration)
	return true
}
