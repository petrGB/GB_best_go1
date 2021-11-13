package requester

import (
	"context"
	"net/http"
	"time"

	"lesson1/internal/page"
)

type Requester interface {
	Get(ctx context.Context, url string) (page.Page, error)
}

type requester struct {
	timeout time.Duration
}

func NewRequester(timeout time.Duration) requester {
	return requester{timeout: timeout}
}

func (r requester) Get(ctx context.Context, url string) (page.Page, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		cl := &http.Client{
			Timeout: r.timeout,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		body, err := cl.Do(req)
		if err != nil {
			return nil, err
		}
		defer body.Body.Close()
		page, err := page.NewPage(url, body.Body)
		if err != nil {
			return nil, err
		}
		return page, nil
	}

}
