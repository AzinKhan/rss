package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

const (
	feedsFile = "feeds.txt"
	maxAge    = 24 * time.Hour
)

var (
	dateFormats = []string{time.RFC1123, time.RFC1123Z}
	client      = http.DefaultClient
	paywalls    = []string{
		"https://www.ft.com",
		"https://rss.nytimes.com",
	}
)

type Feed struct {
	URL string
	RSS
}

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	XMLName     xml.Name `xml:"channel"`
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	Generator   string   `xml:"generator"`
	Language    string   `xml:"language"`
	Items       []Item   `xml:"item"`
}

type Item struct {
	XMLName xml.Name `xml:"item"`
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	PubDate string   `xml:"pubDate"`
	GUID    string   `xml:"guid"`
	// Comments provide a link to a dedicated comments page e.g. hackernews
	Comments    string `xml:"comments"`
	Description []byte `xml:"description"`
}

func main() {
	f, err := os.Open(feedsFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	scanner := bufio.NewScanner(f)
	var urls []string
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	var wg sync.WaitGroup
	wg.Add(len(urls))
	results := make(chan *Feed, len(urls))
	errs := make(chan error, len(urls))
	for _, url := range urls {
		url := url
		go func() {
			defer wg.Done()
			result, err := getFeed(url)
			errs <- err
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for result := range results {
		if result == nil {
			continue
		}
		fmt.Fprintf(w, formatFeed(result))
	}
	w.Flush()

	for err := range errs {
		if err == nil {
			continue
		}
		fmt.Fprintf(w, err.Error())
	}
	w.Flush()
}

func getFeed(url string) (*Feed, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error getting %s: %w", url, err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body from %s: %w", url, err)
	}
	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling body from %s: %w", url, err)
	}
	return &Feed{url, rss}, nil
}

func formatFeed(feed *Feed) string {
	builder := strings.Builder{}
	var hasPaywall bool
	for _, pw := range paywalls {
		if strings.HasPrefix(feed.URL, pw) {
			hasPaywall = true
			break
		}
	}

	formatLink := linkFormatter(hasPaywall)

	var writtenTitle bool
	for _, item := range feed.Channel.Items {
		if !checkAge(item) {
			continue
		}
		if !writtenTitle {
			builder.WriteString(fmt.Sprintf("\n%s\n", feed.Channel.Title))
			writtenTitle = true
		}
		builder.WriteString(formatItem(item, formatLink))
	}
	return builder.String()
}

func checkAge(item Item) bool {
	itemDate, err := parseDate(item.PubDate)
	if err != nil {
		return true
	}
	return time.Since(itemDate) < maxAge
}

func linkFormatter(hasPaywall bool) func(Item) string {
	return func(item Item) string {
		link := item.Link
		if link == "" {
			link = item.GUID
		}
		// Add archive to paywalled links
		if hasPaywall {
			return fmt.Sprintf("https://archive.is/%s", link)
		}
		return link
	}
}

func formatItem(item Item, formatLink func(Item) string) string {
	return fmt.Sprintf("%s:\t%s\t%s\t%s\n", formatDate(item.PubDate), item.Title, formatLink(item), item.Comments)
}

func parseDate(rawDate string) (time.Time, error) {
	var t time.Time
	var err error
	for _, format := range dateFormats {
		t, err = time.Parse(format, rawDate)
		if err == nil {
			break
		}
	}
	return t, err
}

// formatDate attempts to put the rawDate into one of the supported formats. If
// that cannot be done, then the original raw string is returned.
func formatDate(rawDate string) string {
	t, err := parseDate(rawDate)
	if err != nil {
		return rawDate
	}
	y, m, d := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", y, m, d)
}
