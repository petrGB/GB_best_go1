package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lesson1/internal/config"
	"lesson1/internal/crawler"
	"lesson1/internal/requester"
)

func main() {

	log.Printf("My pid: %d\n", os.Getpid())

	var configPath string
	flag.StringVar(&configPath, "config", "config.json", "path to config json file")

	flag.Parse()

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		flag.PrintDefaults()
		panic(err)
	}

	var cr crawler.Crawler
	var r requester.Requester

	r = requester.NewRequester(time.Duration(cfg.RequestTimeout) * time.Second)
	cr = crawler.NewCrawler(r)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout*int(time.Second)))

	go cr.Scan(ctx, cfg.Url, cfg.MaxDepth) //Запускаем краулер в отдельной рутине
	go processResult(ctx, cancel, cr, cfg) //Обрабатываем результаты в отдельной рутине

	sigCh := make(chan os.Signal) //Создаем канал для приема сигналов
	signal.Notify(sigCh,
		syscall.SIGINT,  //Подписываемся на сигнал SIGINT
		syscall.SIGUSR1, //Подписываемся на сигнал SIGUSR1
		syscall.SIGUSR2, //Подписываемся на сигнал SIGUSR2
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

func processResult(ctx context.Context, cancel func(), cr crawler.Crawler, cfg config.Config) {
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
				cancel()
				return
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
