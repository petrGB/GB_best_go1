package page

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var html = `<!doctype html>

<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">

  <title>Test Template</title>
  <meta name="description" content="A simple HTML5 Template for new projects.">
  <meta name="author" content="SitePoint">

</head>

<body>
  <a href="https://test.com">Test</a>
  <a href="/?value=ok">Test2</a>
  <a href="//test.com?value=t">Test3</a>
  <script src="js/scripts.js"></script>
</body>
</html>`

var htmlBad = `<!doctype html>

<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">

  <notitle>Test Template</notitle>
  <meta name="description" content="A simple HTML5 Template for new projects.">
  <meta name="author" content="SitePoint">

</head>

<body>
  <a url="https://test.com">Test</a>
  <p href="https://test.com">Test2</p>
  <script src="js/scripts.js"></script>
</body>
</html>`

func TestNewPage(t *testing.T) {
	_, err := NewPage("https://ya.ru/?param=test", strings.NewReader(html))
	assert.NoError(t, err)
}

type PageTestSuite struct {
	suite.Suite
	page    Page
	pageBad Page
}

func (s *PageTestSuite) SetupTest() {
	s.page, _ = NewPage("https://ya.ru/?param=test", strings.NewReader(html))
	s.pageBad, _ = NewPage("https://ya.ru/?param=test", strings.NewReader(htmlBad))
}

func (s *PageTestSuite) TestMakeFullUrl() {
	s.T().Run("equal test", func(t *testing.T) {
		s.Assert().Equal("https://ya.ru/folder/values/?value=5", s.page.makeFullUrl("folder/values/?value=5"))
		s.Assert().Equal("https://ya.ru/?value=5", s.page.makeFullUrl("?value=5"))
		s.Assert().Equal("https://ya.ru/?value=5", s.page.makeFullUrl("/?value=5"))
		s.Assert().Equal("https://test.com/?value=5", s.page.makeFullUrl("//test.com/?value=5"))
		s.Assert().Equal("http://google.com?value=5", s.page.makeFullUrl("http://google.com?value=5"))
	})

	s.T().Run("not equal test", func(t *testing.T) {
		s.Assert().NotEqual("https://ya.ru/folder/values/?value=5", s.page.makeFullUrl("folder/values/?value=0"))
		s.Assert().NotEqual("https://ya.ru/?value=5", s.page.makeFullUrl("?value=0"))
		s.Assert().NotEqual("https://ya.ru/?value=5", s.page.makeFullUrl("/?value=0"))
		s.Assert().NotEqual("https://ya.ru/?value=5", s.page.makeFullUrl("//?value=0"))
		s.Assert().NotEqual("http://google.com?value=5", s.page.makeFullUrl("http://google.com?value=0"))
	})
}

func (s *PageTestSuite) TestGetTitle() {
	s.T().Run("good html test", func(t *testing.T) {
		s.Assert().Equal("Test Template", s.page.GetTitle())
		s.Assert().NotEqual("", s.page.GetTitle())
	})

	s.T().Run("bad html test", func(t *testing.T) {
		s.Assert().NotEqual("Test Template", s.pageBad.GetTitle())
		s.Assert().Equal("", s.pageBad.GetTitle())
	})
}

func (s *PageTestSuite) TestGetLinks() {
	s.T().Run("good html test", func(t *testing.T) {
		links := []string{"https://test.com", "https://ya.ru/?value=ok", "https://test.com?value=t"}
		s.Assert().Equal(links, s.page.GetLinks())
	})

	s.T().Run("bad html test", func(t *testing.T) {
		links := []string(nil)
		s.Assert().Equal(links, s.pageBad.GetLinks())
		links = []string{"https://test.com", "https://ya.ru/?value=ok", "https://test.com?value=t"}
		s.Assert().NotEqual(links, s.pageBad.GetLinks())
	})
}

func TestPageTestSuite(t *testing.T) {
	suite.Run(t, new(PageTestSuite))
}
