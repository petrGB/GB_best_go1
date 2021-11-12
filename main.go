package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type CrawlResult struct {
	Err   error
	Title string
	Url   string
}

type Page interface {
	GetTitle() string
	GetLinks() []string
	MakeFullUrl(string) string
}

type page struct {
	url string
	doc *goquery.Document
}

func NewPage(url string, raw io.Reader) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		return nil, err
	}
	return &page{url: url, doc: doc}, nil
}

func (p *page) MakeFullUrl(shortUrl string) string {
	u, err := url.Parse(p.url)
	if err != nil {
		return ""
	}

	//собираем полный url
	if strings.Index(shortUrl, "http") != 0 {
		if u.User != nil {
			shortUrl = fmt.Sprintf("%s://%s@%s", u.Scheme, u.User, u.Host)
		} else {
			shortUrl = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}

	return shortUrl
}

func (p *page) GetTitle() string {
	return p.doc.Find("title").First().Text()
}

func (p *page) GetLinks() []string {
	var urls []string
	p.doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		url, ok := s.Attr("href")
		if ok {
			url = p.MakeFullUrl(url)
			urls = append(urls, url)
		}
	})
	return urls
}

type Requester interface {
	Get(ctx context.Context, url string) (Page, error)
}

type requester struct {
	timeout time.Duration
}

func NewRequester(timeout time.Duration) requester {
	return requester{timeout: timeout}
}

func (r requester) Get(ctx context.Context, url string) (Page, error) {
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
		page, err := NewPage(url, body.Body)
		if err != nil {
			return nil, err
		}
		return page, nil
	}

}

//Crawler - интерфейс (контракт) краулера
type Crawler interface {
	Scan(ctx context.Context, wg *sync.WaitGroup, url string, depth int)
	ChanResult() <-chan CrawlResult
}

type crawler struct {
	r       Requester
	res     chan CrawlResult
	visited map[string]struct{}
	mu      sync.RWMutex
}

func NewCrawler(r Requester) *crawler {
	return &crawler{
		r:       r,
		res:     make(chan CrawlResult),
		visited: make(map[string]struct{}),
		mu:      sync.RWMutex{},
	}
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
			go c.Scan(ctx, wg, link, depth-1) //На все полученные ссылки запускаем новую рутину сборки
		}
	}
}

func (c *crawler) ChanResult() <-chan CrawlResult {
	return c.res
}

//Config - структура для конфигурации
type Config struct {
	MaxDepth       int
	MaxResults     int
	MaxErrors      int
	Url            string
	RequestTimeout int //in seconds
	AppTimeout     int //in seconds
}

func main() {

	cfg := Config{
		MaxDepth:       1,
		MaxResults:     10,
		MaxErrors:      5,
		Url:            "https://telegram.org",
		RequestTimeout: 10,
		AppTimeout:     60,
	}
	var cr Crawler
	var r Requester

	r = NewRequester(time.Duration(cfg.RequestTimeout) * time.Second)
	cr = NewCrawler(r)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout*int(time.Second)))
	var wg sync.WaitGroup
	wg.Add(1)
	go cr.Scan(ctx, &wg, cfg.Url, cfg.MaxDepth) //Запускаем краулер в отдельной рутине
	go processResult(ctx, cancel, cr, cfg)      //Обрабатываем результаты в отдельной рутине
	go func() {
		wg.Wait()
		cancel() //все сканы завершились, а maxResult не достигнут
	}()

	sigCh := make(chan os.Signal)        //Создаем канал для приема сигналов
	signal.Notify(sigCh, syscall.SIGINT) //Подписываемся на сигнал SIGINT
	for {
		select {
		case <-ctx.Done(): //Если всё завершили - выходим
			return
		case <-sigCh:
			cancel() //Если пришёл сигнал SigInt - завершаем контекст
		}
	}
}

func processResult(ctx context.Context, cancel func(), cr Crawler, cfg Config) {
	var maxResult, maxErrors = cfg.MaxResults, cfg.MaxErrors
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-cr.ChanResult():
			if msg.Err != nil {
				maxErrors--
				log.Printf("crawler result return err: %s\n", msg.Err.Error())
				if maxErrors <= 0 {
					cancel()
					return
				}
			} else {
				maxResult--
				log.Printf("crawler result: [url: %s] Title: %s\n", msg.Url, msg.Title)
				if maxResult <= 0 {
					cancel()
					return
				}
			}
		}
	}
}
