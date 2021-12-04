package page

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Page interface {
	GetTitle() string
	GetLinks() []string
	makeFullUrl(string) string
}

type page struct {
	url        string
	mainUrl    string
	mainScheme string
	doc        *goquery.Document
}

func NewPage(inUrl string, raw io.Reader) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(raw)
	if err != nil {
		return nil, err
	}

	//сохраняем основной url сайта
	mainUrl := inUrl
	mainScheme := "http"
	u, err := url.Parse(inUrl)
	if err == nil {
		if u.User != nil {
			mainUrl = fmt.Sprintf("%s://%s@%s", u.Scheme, u.User, u.Host)
		} else {
			mainUrl = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
		mainScheme = u.Scheme
	}
	mainUrl = strings.TrimRight(mainUrl, "/")

	return &page{url: inUrl, mainUrl: mainUrl, mainScheme: mainScheme, doc: doc}, nil
}

func (p *page) makeFullUrl(shortUrl string) string {
	if strings.Index(shortUrl, "//") == 0 {
		return p.mainScheme + "://" + strings.TrimLeft(shortUrl, "/")
	} else if strings.Index(shortUrl, "http") != 0 {
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
