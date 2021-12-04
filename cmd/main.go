package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"lesson1/internal/config"
	"lesson1/internal/crawler"
	"lesson1/internal/requester"
)

func main() {

	var configPath string
	flag.StringVar(&configPath, "config", "config.json", "path to config json file")

	flag.Parse()

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		flag.PrintDefaults()
		panic(err)
	}

	logCf := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.Level(cfg.LogLevel)),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	zlog, _ := logCf.Build()
	//Error return value of `log.Sync` is not checked (errcheck)
	defer func() {
		err := zlog.Sync() // flushes buffer, if any
		if err != nil {
			log.Println(err)
		}
	}()

	plog := zlog.With(zap.Int("pid", os.Getpid()))

	plog.Debug("start")

	var cr crawler.Crawler
	var r requester.Requester

	r = requester.NewRequester(time.Duration(cfg.RequestTimeout)*time.Second, plog)
	cr = crawler.NewCrawler(r, plog)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AppTimeout*int(time.Second)))

	go cr.Scan(ctx, cfg.Url, cfg.MaxDepth)       //Запускаем краулер в отдельной рутине
	go processResult(ctx, cancel, cr, cfg, plog) //Обрабатываем результаты в отдельной рутине

	//sigchanyzer: misuse of unbuffered os.Signal channel as argument to signal.Notify (govet)
	sigCh := make(chan os.Signal, 1) //Создаем канал для приема сигналов
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
				plog.Info("received syscall SIGUSR1")
				cr.DepthDiff(2)
			} else if sig == syscall.SIGUSR2 {
				plog.Info("received syscall SIGUSR2")
				cr.DepthDiff(-2)
			} else {
				cancel() //Если пришёл сигнал SigInt - завершаем контекст
			}
		}
	}
}

func processResult(ctx context.Context, cancel func(), cr crawler.Crawler, cfg config.Config, log *zap.Logger) {
	var maxResult, maxErrors = cfg.MaxResults, cfg.MaxErrors
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-cr.ChanResult():
			if msg.Err != nil {
				maxErrors--
				log.Info("crawler result", zap.String("err", msg.Err.Error()))
				if maxErrors <= 0 {
					cancel()
					return
				}
			} else if msg.Info != "" {
				log.Info("crawler result", zap.String("info", msg.Info))
				cancel()
				return
			} else {
				maxResult--
				log.Info("crawler result", zap.String("url", msg.Url), zap.String("title", msg.Title))
				if maxResult <= 0 {
					cancel()
					return
				}
			}
		}
	}
}
