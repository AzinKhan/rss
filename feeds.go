package rss

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const outputTimeLayout = "2006/01/02"

var (
	dateFormats = []string{time.RFC1123, time.RFC1123Z}
	client      = http.DefaultClient
	paywalls    = []string{
		"https://www.ft.com",
		"https://rss.nytimes.com",
	}
)

type FeedItem struct {
	Title       string
	PublishTime time.Time
	Links       []string
	Feed        string
}

func (fi FeedItem) Format() string {
	builder := strings.Builder{}
	if !fi.PublishTime.IsZero() {
		date := fi.PublishTime.Format(outputTimeLayout)
		builder.WriteString(fmt.Sprintf("%s:", date))
	}

	builder.WriteString(fmt.Sprintf("\t%s", fi.Title))
	for _, link := range fi.Links {
		builder.WriteString(fmt.Sprintf("\t%s", link))
	}
	builder.WriteString("\n")
	return builder.String()
}

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

type DisplayMode func([]FeedItem) []FeedItem

func ReverseChronological(feedItems []FeedItem) []FeedItem {
	sort.Slice(feedItems, func(i, j int) bool {
		return feedItems[i].PublishTime.After(feedItems[j].PublishTime)
	})
	return feedItems
}

func Grouped(feedItems []FeedItem) []FeedItem {
	itemsByFeed := make(map[string][]FeedItem)
	for _, item := range feedItems {
		existing := itemsByFeed[item.Feed]
		existing = append(existing, item)
		itemsByFeed[item.Feed] = existing
	}

	result := make([]FeedItem, 0, len(feedItems))
	for feed, items := range itemsByFeed {
		if len(items) == 0 {
			continue
		}
		// Create a title-only item for the feed itself
		result = append(result, FeedItem{})
		result = append(result, FeedItem{Title: feed})
		for _, item := range ReverseChronological(items) {
			result = append(result, item)
		}
	}
	return result
}

// Display writes the feed items to the given writer in the provided display
// mode.
func Display(w io.Writer, feedItems []FeedItem, displayMode DisplayMode) {
	for _, item := range displayMode(feedItems) {
		fmt.Fprintf(w, item.Format())
	}
}

type Filter func(FeedItem) bool

// Deduplicate ensures that each feed item only appears in the output once.
func Deduplicate() Filter {
	urls := make(map[string]struct{})
	return func(item FeedItem) bool {
		for _, link := range item.Links {
			_, found := urls[link]
			if found {
				return false
			}
			urls[link] = struct{}{}
		}
		return true
	}
}

// OldestItem ensures that the output feed items are less than the max age
// given.
func OldestItem(maxAge time.Duration) Filter {
	return func(item FeedItem) bool {
		return time.Since(item.PublishTime) <= maxAge
	}
}

// GetFeedItems unpacks the items within the given feeds, applying filters if
// given.
func GetFeedItems(feeds []*Feed, filters ...Filter) []FeedItem {
	var feedItems []FeedItem
	for _, feed := range feeds {
		if feed == nil {
			continue
		}
		newFeedItem := newFeedItemCreator(feed)
	itemloop:
		for _, item := range feed.Channel.Items {
			feedItem, err := newFeedItem(item)
			if err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
				continue
			}
			for _, filter := range filters {
				if !filter(feedItem) {
					continue itemloop
				}
			}
			feedItems = append(feedItems, feedItem)
		}
	}
	return feedItems
}

// GetURLs reads the given Reader and returns a list of the urls from which
// feeds can be fetched.
func GetURLs(r io.Reader) []string {
	scanner := bufio.NewScanner(r)
	var urls []string
	for scanner.Scan() {
		url := scanner.Text()
		if strings.HasPrefix(url, "#") {
			// Commented out url
			continue
		}
		urls = append(urls, url)
	}
	return urls
}

// GetFeeds makes requests to the hosts at the given URLs and returns a *Feed
// for each.
func GetFeeds(urls []string) []*Feed {
	var wg sync.WaitGroup
	wg.Add(len(urls))
	results := make([]*Feed, len(urls))
	for i, url := range urls {
		url := url
		i := i
		go func() {
			defer wg.Done()
			result, err := getFeed(url)
			if err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
			}
			results[i] = result
		}()
	}
	wg.Wait()
	return results
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

func linkFormatter(feed *Feed) func(Item) string {
	var hasPaywall bool
	for _, pw := range paywalls {
		if strings.HasPrefix(feed.URL, pw) {
			hasPaywall = true
			break
		}
	}
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

func newFeedItemCreator(feed *Feed) func(Item) (FeedItem, error) {
	return func(item Item) (FeedItem, error) {
		formatLink := linkFormatter(feed)
		links := []string{formatLink(item)}
		if item.Comments != "" {
			links = append(links, item.Comments)
		}
		pubTime, err := parseDate(item.PubDate)
		if err != nil {
			return FeedItem{}, err
		}
		return FeedItem{
			Title:       item.Title,
			Links:       links,
			PublishTime: pubTime,
			Feed:        feed.Channel.Title,
		}, nil
	}
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
