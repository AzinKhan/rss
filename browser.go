package rss

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type Browser struct {
	pw *playwright.Playwright
	b  playwright.Browser
}

func NewBrowser() (*Browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	b, err := pw.Firefox.Launch()
	if err != nil {
		return nil, err
	}

	return &Browser{
		pw: pw,
		b:  b,
	}, nil
}

func (b *Browser) Stop() {
	b.b.Close()
	b.pw.Stop()
}

// WriteText fetches the page at the given URL and writes its text to the given
// Writer.
func (b *Browser) WriteText(url string, w io.Writer) error {
	page, err := b.b.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %v", err)
	}
	_, err = page.Goto(fmt.Sprintf("about:reader?url=%s", url))
	if err != nil {
		return fmt.Errorf("could not goto: %v", err)
	}
	// Need to wait for the reader mode to take effect
	time.Sleep(1 * time.Second)

	entries, err := page.QuerySelectorAll("div[class='moz-reader-content reader-show-element']")
	if err != nil {
		return fmt.Errorf("could not get entries: %v", err)
	}

	wrapLines := newLineWrapper(72)
	for _, entry := range entries {
		titleElement, err := entry.QuerySelector("h3")
		if err != nil {
			fmt.Println(err)
			continue
		}
		if titleElement != nil {
			title, err := titleElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Fprintf(w, title)
		}
		bodyElements, err := entry.QuerySelectorAll("p")
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, bodyElement := range bodyElements {
			body, err := bodyElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, line := range wrapLines(body) {
				fmt.Fprintf(w, "\t%s\n", strings.TrimSpace(line))
			}
			fmt.Fprintf(w, "\n")
		}
	}
	return nil
}

type Page struct {
	*bytes.Buffer
}

func (b *Browser) NewPage(url string) (*Page, error) {
	page, err := b.b.NewPage()
	if err != nil {
		return nil, fmt.Errorf("could not create page: %v", err)
	}
	_, err = page.Goto(fmt.Sprintf("about:reader?url=%s", url))
	if err != nil {
		return nil, fmt.Errorf("could not goto: %v", err)
	}
	// Need to wait for the reader mode to take effect
	time.Sleep(1 * time.Second)

	entries, err := page.QuerySelectorAll("div[class='moz-reader-content reader-show-element']")
	if err != nil {
		return nil, fmt.Errorf("could not get entries: %v", err)
	}

	wrapLines := newLineWrapper(72)
	var p []byte
	w := bytes.NewBuffer(p)
	for _, entry := range entries {
		titleElement, err := entry.QuerySelector("h3")
		if err != nil {
			fmt.Println(err)
			continue
		}
		if titleElement != nil {
			title, err := titleElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Fprintf(w, title)
		}
		bodyElements, err := entry.QuerySelectorAll("p")
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, bodyElement := range bodyElements {
			body, err := bodyElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, line := range wrapLines(body) {
				fmt.Fprintf(w, "\t%s\n", strings.TrimSpace(line))
			}
			fmt.Fprintf(w, "\n")
		}
	}

	return &Page{w}, nil

}

func newLineWrapper(softLimit int) func(string) []string {
	return func(body string) []string {
		var result []string
		for len(body) > softLimit {
			delimiter := body[softLimit]
			lineBreakIdx := softLimit
			if string(delimiter) != " " {
				// Find the next space
				lineBreakIdx += findIndexWhere(body[lineBreakIdx:], " ")
			}
			line := body[:lineBreakIdx]
			result = append(result, line)
			body = body[lineBreakIdx:]
		}
		if len(body) > 0 {
			result = append(result, body)
		}
		return result

	}
}

func findIndexWhere(s string, target string) int {
	for i, char := range s {
		if string(char) != target {
			continue
		}
		return i
	}
	return 0
}
