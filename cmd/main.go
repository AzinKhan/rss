package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"text/tabwriter"
	"time"

	"github.com/AzinKhan/rss"
)

const (
	feedsFile = ".rss/feeds.txt"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Expected a subcommand")
		os.Exit(1)
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	feedsFilepath := path.Join(homedir, feedsFile)

	var displayMode rss.DisplayMode
	command := os.Args[1]
	switch command {
	case "edit":
		err := editFeedsFile(feedsFilepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "feed":
		displayMode = rss.ReverseChronological
	case "group":
		displayMode = rss.Grouped
	default:
		fmt.Printf("Unknown command %s\n", command)
		os.Exit(1)
	}

	var maxHours int
	flag.IntVar(&maxHours, "h", 24, "Max age of items (hours)")
	flag.Parse()

	maxAge := time.Duration(maxHours) * time.Hour

	f, err := os.Open(feedsFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
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
