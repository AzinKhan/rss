package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/AzinKhan/rss"
)

const (
	feedsFile = "feeds.txt"
)

func main() {
	var dm, maxHours int
	flag.IntVar(&dm, "m", 0, "Display mode")
	flag.IntVar(&maxHours, "h", 24, "Max age of items (hours)")
	flag.Parse()
	displayMode := rss.DisplayMode(dm)
	maxAge := time.Duration(maxHours) * time.Hour

	f, err := os.Open(feedsFile)
	if err != nil {
		fmt.Println(err)
		return
	}

	feeds := rss.GetFeeds(rss.GetURLs(f))
	feedItems := rss.GetFeedItems(feeds, rss.OldestItem(maxAge), rss.Deduplicate())

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for _, item := range rss.Display(feedItems, displayMode) {
		fmt.Fprintf(w, item.Format())
	}
	w.Flush()
}
