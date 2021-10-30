package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	urls = []string{
		"https://danluu.com/atom",
		"http://syndication.thedailywtf.com/TheDailyWtf",
		"https://www.ft.com/world?format=rss",
	}
	dateFormats = []string{time.RFC1123, time.RFC1123Z}
	client      = http.DefaultClient
)

type Feed struct {
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
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	GUID        string   `xml:"guid"`
	Description []byte   `xml:"description"`
}

func main() {

	var results []*Feed
	for _, url := range urls {
		result, err := getFeed(url)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, result := range results {
		fmt.Fprintf(w, formatFeed(result))
	}
	w.Flush()
}

func formatFeed(feed *Feed) string {
	builder := strings.Builder{}
	for _, item := range feed.Channel.Items {
		builder.WriteString(formatItem(item))
	}
	return builder.String()
}

func formatItem(item Item) string {
	return fmt.Sprintf("%s:\t%s\t%s\n", formatDate(item.PubDate), item.Title, item.Link)
}

func getFeed(url string) (*Feed, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var feed Feed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, err
	}
	return &feed, nil
}

// formatDate attempts to put the rawDate into one of the supported formats. If
// that cannot be done, then the original raw string is returned.
func formatDate(rawDate string) string {
	var t time.Time
	var err error
	for _, format := range dateFormats {
		t, err = time.Parse(format, rawDate)
		if err == nil {
			break
		}
	}
	if err != nil {
		return rawDate
	}
	y, m, d := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", y, m, d)
}
