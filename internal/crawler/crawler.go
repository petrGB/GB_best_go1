package crawler

import (
	"context"
	"sync"
	"sync/atomic"

	"lesson1/internal/requester"
)

type CrawlResult struct {
	Err   error
	Info  string
	Title string
	Url   string
}

//Crawler - интерфейс (контракт) краулера
type Crawler interface {
	Scan(ctx context.Context, wg *sync.WaitGroup, url string, depth int)
	ChanResult() <-chan CrawlResult
	ToChanResult(CrawlResult)
	DepthDiff(int32)
}

type crawler struct {
	r         requester.Requester
	res       chan CrawlResult
	visited   map[string]struct{}
	mu        sync.RWMutex
	depthDiff int32 // для изменения depth
}

func NewCrawler(r requester.Requester) *crawler {
	return &crawler{
		r:         r,
		res:       make(chan CrawlResult),
		visited:   make(map[string]struct{}),
		mu:        sync.RWMutex{},
		depthDiff: 0,
	}
}

func (c *crawler) DepthDiff(diff int32) {
	atomic.AddInt32(&c.depthDiff, diff)
}

func (c *crawler) Scan(ctx context.Context, wg *sync.WaitGroup, url string, depth int) {
	defer wg.Done()

	if depth <= 0 { //Проверяем то, что есть запас по глубине
		return
	}

	c.mu.Lock()
	_, ok := c.visited[url] //Проверяем, что мы ещё не смотрели эту страницу
	if ok {
		c.mu.Unlock()
		return
	}
	c.visited[url] = struct{}{} //Помечаем страницу просмотренной (еще до того как посмотрели, чтоб другие грутины не пытались паралельно)
	c.mu.Unlock()

	select {
	case <-ctx.Done(): //Если контекст завершен - прекращаем выполнение
		return
	default:
		page, err := c.r.Get(ctx, url) //Запрашиваем страницу через Requester
		if err != nil {
			c.res <- CrawlResult{Err: err} //Записываем ошибку в канал
			return
		}
		c.res <- CrawlResult{ //Отправляем результаты в канал
			Title: page.GetTitle(),
			Url:   url,
		}
		for _, link := range page.GetLinks() {
			wg.Add(1)
			go c.Scan(ctx, wg, link, int(c.depthDiff)+depth-1) //На все полученные ссылки запускаем новую рутину сборки
		}
	}
}

func (c *crawler) ChanResult() <-chan CrawlResult {
	return c.res
}

func (c *crawler) ToChanResult(crawResult CrawlResult) {
	c.res <- crawResult
}
