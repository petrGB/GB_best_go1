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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type CrawlResult struct {
	Err   error
	Info  string
	Title string
	Url   string
}

type Page interface {
	GetTitle() string
	GetLinks() []string
	makeFullUrl(string) string
}

type page struct {
	url     string
	mainUrl string
	doc     *goquery.Document
}

func NewPage(inUrl string, raw io.Reader) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		return nil, err
	}

	//сохраняем основной url сайта
	mainUrl := inUrl
	u, err := url.Parse(inUrl)
	if err == nil {
		if u.User != nil {
			mainUrl = fmt.Sprintf("%s://%s@%s", u.Scheme, u.User, u.Host)
		} else {
			mainUrl = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}
	mainUrl = strings.TrimRight(mainUrl, "/")

	return &page{url: inUrl, mainUrl: mainUrl, doc: doc}, nil
}

func (p *page) makeFullUrl(shortUrl string) string {
	if strings.Index(shortUrl, "http") != 0 {
		shortUrl = strings.TrimLeft(shortUrl, "/")
		return p.mainUrl + "/" + shortUrl
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
			url = p.makeFullUrl(url)
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
	ToChanResult(CrawlResult)
	DepthDiff(int32)
}

type crawler struct {
	r         Requester
	res       chan CrawlResult
	visited   map[string]struct{}
	mu        sync.RWMutex
	depthDiff int32 // для изменения depth
}

func NewCrawler(r Requester) *crawler {
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
			c.mu.RLock()
			go c.Scan(ctx, wg, link, int(c.depthDiff)+depth-1) //На все полученные ссылки запускаем новую рутину сборки
			c.mu.RUnlock()
		}
	}
}

func (c *crawler) ChanResult() <-chan CrawlResult {
	return c.res
}

func (c *crawler) ToChanResult(crawResult CrawlResult) {
	c.res <- crawResult
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

	log.Printf("My pid: %d\n", os.Getpid())

	cfg := Config{
		MaxDepth:       5,
		MaxResults:     50,
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
		cr.ToChanResult(CrawlResult{
			Info: "All urls already scanned", //Записываем сообщение канал
		})
		cancel() //все сканы завершились, а maxResult не достигнут
	}()

	sigCh := make(chan os.Signal) //Создаем канал для приема сигналов
	signal.Notify(sigCh,
		syscall.SIGINT,  //Подписываемся на сигнал SIGINT
		syscall.SIGUSR1, //Подписываемся на сигнал SIGUSR1
	)
	for {
		select {
		case <-ctx.Done(): //Если всё завершили - выходим
			return
		case sig := <-sigCh:
			if sig == syscall.SIGUSR1 {
				log.Println("received syscall SIGUSR1")
				cr.DepthDiff(2)
			} else if sig == syscall.SIGUSR2 {
				log.Println("received syscall SIGUSR2")
				cr.DepthDiff(-2)
			} else {
				cancel() //Если пришёл сигнал SigInt - завершаем контекст
			}
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
			} else if msg.Info != "" {
				log.Printf("crawler result return info: %s\n", msg.Info)
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
