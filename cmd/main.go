package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"
	"time"

	"github.com/AzinKhan/rss"
)

const (
	feedsFile = "feeds.txt"
)

func main() {
	var dm, maxHours int
	var edit bool
	flag.IntVar(&dm, "m", 0, "Display mode")
	flag.IntVar(&maxHours, "h", 24, "Max age of items (hours)")
	flag.BoolVar(&edit, "e", false, "Edit the feeds list")
	flag.Parse()
	displayMode := rss.ReverseChronological
	if dm == 1 {
		displayMode = rss.Grouped
	}
	maxAge := time.Duration(maxHours) * time.Hour

	if edit {
		err := editFeedsFile(feedsFile)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	f, err := os.Open(feedsFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	feeds := rss.GetFeeds(rss.GetURLs(f))
	feedItems := rss.GetFeedItems(feeds, rss.OldestItem(maxAge), rss.Deduplicate())

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	rss.Display(w, feedItems, displayMode)
	w.Flush()
}

func editFeedsFile(filepath string) error {
	cmd := exec.Command("vim", filepath)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
