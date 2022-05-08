package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
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
	f, err := os.Open(feedsFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer f.Close()
	urls := rss.GetURLs(f)

	var displayMode rss.DisplayMode
	itemFilter := rss.MaxItemsPerChannel

	var interactive bool
	flag.BoolVar(&interactive, "i", false, "Enable interactive mode")

	flag.Parse()

	command := os.Args[1]
	if interactive {
		command = os.Args[2]
	}
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
		itemFilter = rss.MaxItems
	case "group":
		displayMode = rss.Grouped
	case "select":
		urls = []string{selectSingleFeed(urls)}
		displayMode = rss.ReverseChronological
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

	feedsCh := rss.RefreshFeeds(urls)

	filters := []rss.Filter{rss.OldestItem(maxAge), rss.Deduplicate(), itemFilter(maxItems)}

	if interactive {
		browser, err := rss.NewBrowser()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		err = interactiveDisplay(feedsCh, browser, displayMode, rss.WithFilters(filters...))
	} else {
		var feeds []*rss.Feed
		for feed := range feedsCh {
			feeds = append(feeds, feed)
		}
		feedItems := rss.GetFeedItems(feeds, filters...)
		now := time.Now()
		err = display(feedItems, displayMode, rss.ColourAfter(now.Add(-2*time.Hour)))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func selectSingleFeed(urls []string) string {
	n := len(urls)
	numPlaces := n / 10
	printSelection(urls)
	var i int
	for {

		b := make([]byte, 1+numPlaces)
		os.Stdin.Read(b)

		iStr := strings.Split(string(b), "\n")
		if len(iStr) < 1 {
			continue
		}

		var err error
		i, err = strconv.Atoi(iStr[0])
		if err != nil {
			fmt.Println(err)
			continue
		}

		if i < n {
			break
		}

	}
	return urls[i]

}

func printSelection(urls []string) {
	var builder strings.Builder
	for i, url := range urls {
		builder.WriteString(fmt.Sprintf("%d:\t", i))
		builder.WriteString(url)
		builder.WriteString("\n")
	}
	fmt.Fprintf(os.Stdout, builder.String())
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

func interactiveDisplay(feeds chan *rss.Feed, b *rss.Browser, mode rss.DisplayMode, opts ...rss.AppOption) error {
	app := rss.NewApp(feeds, b, mode, opts...)
	return app.Run()
}
