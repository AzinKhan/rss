package rss

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/AzinKhan/functools"
)

type Colour string

const (
	outputTimeLayout = "2006/01/02"
	// Note these colour codes are not supported on windows.
	reset  Colour = "\033[0m"
	red    Colour = "\033[31m"
	green  Colour = "\033[32m"
	yellow Colour = "\033[33m"
	blue   Colour = "\033[34m"
	purple Colour = "\033[35m"
	cyan   Colour = "\033[36m"
	gray   Colour = "\033[37m"
	white  Colour = "\033[97m"
)

var (
	dateFormats = []string{time.RFC1123, time.RFC1123Z, "Mon, 2 Jan 2006 15:04:05 MST"}
	client      = http.DefaultClient
	paywalls    = []string{}
)

type FeedItem struct {
	Title       string
	PublishTime time.Time
	Links       []string
	Feed        string
	Channel     string
}

func (fi FeedItem) Format() string {
	return formatFeed(fi, includeLinks(true))
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

type DisplayOption func(FeedItem) FeedItem

func ColourAfter(t time.Time) DisplayOption {
	return func(item FeedItem) FeedItem {
		if item.PublishTime.After(t) {
			item.Title = colourize(item.Title, cyan)
		}
		return item
	}
}

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

	result := make([]FeedItem, 0, len(itemsByFeed))
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
// mode. Returns any error encountered by writing to w.
func Display(w io.Writer, feedItems []FeedItem, displayMode DisplayMode, opts ...DisplayOption) error {
	for _, item := range displayMode(feedItems) {
		for _, o := range opts {
			item = o(item)
		}
		_, err := fmt.Fprintf(w, item.Format())
		if err != nil {
			return err
		}
	}
	return nil
}

type Filter func(FeedItem) bool

type Filters []Filter

// Apply applies all the filters to an item and returns true if they all pass,
// otherwise false.
func (fs Filters) Apply(fi FeedItem) bool {
	for _, f := range fs {
		if !f(fi) {
			return false
		}
	}
	return true
}

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

// MaxItemsPerChannel puts a limit on the number of items per channel. Passing zero in
// results in no limit
func MaxItemsPerChannel(n int) Filter {
	if n == 0 {
		return func(FeedItem) bool { return true }
	}
	counts := make(map[string]int)
	return func(item FeedItem) bool {
		channelCount := counts[item.Channel]
		channelCount++
		counts[item.Channel] = channelCount
		return channelCount <= n
	}
}

// MaxItems enforces a limit on the total number of items in the result.
// Passing zero in results in no limit.
func MaxItems(n int) Filter {
	if n == 0 {
		return func(FeedItem) bool { return true }
	}

	count := 0
	return func(item FeedItem) bool {
		count++
		return count <= n
	}
}

// GetFeedItems unpacks the items within the given feeds, applying filters if
// given.
func GetFeedItems(feeds []*Feed, filters ...Filter) []FeedItem {
	feedItems := make([]FeedItem, 0, len(feeds))
	for _, feed := range feeds {
		if feed == nil {
			continue
		}
		feedItems = append(feedItems, UnpackFeed(feed, filters...)...)
	}
	return feedItems
}

func UnpackFeed(feed *Feed, filters ...Filter) []FeedItem {
	newFeedItem := newFeedItemCreator(feed)
	fs := Filters(filters)

	feedItems := make([]FeedItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		feedItem, err := newFeedItem(item)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			continue
		}
		if !fs.Apply(feedItem) {
			continue
		}

		feedItems = append(feedItems, feedItem)
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

// RefreshFeedsAsync makes requests to the hosts in parallel and writes them to
// the returned channel.
func RefreshFeedsAsync(urls []string) <-chan *Feed {
	return functools.MapChan(getFeed, urls)
}

func RefreshFeeds(urls []string) []*Feed {
	return functools.MapAsync(getFeed, urls)
}

func getFeed(url string) *Feed {
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting %s: %s", url, err.Error())
		return nil
	}
	defer resp.Body.Close()
	var rss RSS
	err = xml.NewDecoder(resp.Body).Decode(&rss)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error unmarshaling body from %s: %s", url, err.Error())
		return nil
	}
	return &Feed{url, rss}
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
		// Clear query params since they're usually just for tracking
		u, err := url.Parse(link)
		if err != nil {
			return err.Error()
		}
		u.RawQuery = ""

		link = u.String()
		// Add archive to paywalled links
		if hasPaywall {
			return fmt.Sprintf("https://archive.is/%s", link)
		}
		return link
	}
}

func newFeedItemCreator(feed *Feed) func(Item) (FeedItem, error) {
	parseDate := newDateParser(time.Now())
	formatLink := linkFormatter(feed)
	return func(item Item) (FeedItem, error) {
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
			Channel:     feed.Channel.Title,
		}, nil
	}
}

func newDateParser(defaultTime time.Time) func(string) (time.Time, error) {
	return func(rawDate string) (time.Time, error) {
		if rawDate == "" {
			return defaultTime, nil
		}
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
}
