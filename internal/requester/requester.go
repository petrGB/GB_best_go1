package requester

import (
	"context"
	"net/http"
	"time"

	"lesson1/internal/page"

	"go.uber.org/zap"
	log "go.uber.org/zap"
)

type Requester interface {
	Get(ctx context.Context, url string) (page.Page, error)
}

type requester struct {
	timeout time.Duration
	log     *log.Logger
}

func NewRequester(timeout time.Duration, log *log.Logger) requester {
	return requester{timeout: timeout, log: log}
}

func (r requester) Get(ctx context.Context, url string) (page.Page, error) {

	r.log.Debug("start", zap.String("url", url))
	defer r.log.Debug("finish", zap.String("url", url))

	select {
	case <-ctx.Done():
		r.log.Debug("ctx done", zap.String("url", url))
		return nil, nil
	default:
		cl := &http.Client{
			Timeout: r.timeout,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			r.log.Error(err.Error(), zap.String("url", url))
			return nil, err
		}
		body, err := cl.Do(req)
		if err != nil {
			r.log.Error(err.Error(), zap.String("url", url))
			return nil, err
		}
		defer body.Body.Close()
		page, err := page.NewPage(url, body.Body)
		if err != nil {
			r.log.Error(err.Error(), zap.String("url", url))
			return nil, err
		}

		return page, nil
	}

}
