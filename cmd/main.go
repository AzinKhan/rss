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
	feedsFile = ".rss/urls.txt"
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

	var maxHours, maxItems int
	args := flag.NewFlagSet("display", flag.ExitOnError)
	args.IntVar(&maxHours, "max", 24, "Max age of items (hours)")
	args.IntVar(&maxItems, "limit", 0, "Max items per channel")
	args.Parse(os.Args[2:])
	maxAge := time.Duration(maxHours) * time.Hour

	f, err := os.Open(feedsFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer f.Close()

	feeds := rss.RefreshFeeds(rss.GetURLs(f))
	feedItems := rss.GetFeedItems(feeds, rss.OldestItem(maxAge), rss.Deduplicate(), rss.MaxItemsPerChannel(maxItems))

	now := time.Now()
	err = display(feedItems, displayMode, rss.ColourAfter(now.Add(-2*time.Hour)))
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func editFeedsFile(filepath string) error {
	cmd := exec.Command("vim", filepath)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func display(feedItems []rss.FeedItem, mode rss.DisplayMode, opts ...rss.DisplayOption) error {
	// Pipe output to less for paging
	cmd := exec.Command("less")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	pipeW, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(pipeW, 1, 1, 1, ' ', 0)
	rss.Display(w, feedItems, mode, opts...)
	err = w.Flush()
	if err != nil {
		return err
	}
	pipeW.Close()
	return cmd.Run()
}
