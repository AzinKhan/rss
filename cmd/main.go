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
	argv := os.Args[2:]
	if interactive {
		argv = os.Args[3:]
	}
	args.Parse(argv)
	maxAge := time.Duration(maxHours) * time.Hour

	filters := []rss.Filter{rss.OldestItem(maxAge), rss.Deduplicate(), itemFilter(maxItems)}

	if interactive {
		feedsCh := rss.GetFeedsAsync(urls)
		err = interactiveDisplay(feedsCh, displayMode, rss.WithFilters(filters...))
	} else {
		feeds := rss.GetFeeds(urls)
		feedItems := rss.GetFeedItems(feeds, filters...)
		now := time.Now()
		err = display(feedItems, displayMode, rss.ColourAfter(now.Add(-2*time.Hour)))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// selectSingleFeed shows the list of urls to the user and allows them to select
// one to load interactively by typing in the corresponding number.
func selectSingleFeed(urls []string) string {
	n := len(urls)
	numDigits := 0
	for N := n; N != 0; N /= 10 {
		numDigits++
	}
	printSelection(urls)
	var i int
	for {

		b := make([]byte, numDigits)
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
		builder.WriteString(fmt.Sprintf("%d:\t%s\n", i, url))
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
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	err := rss.Display(w, feedItems, mode, opts...)
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	return nil
}

func interactiveDisplay(feeds <-chan *rss.Feed, mode rss.DisplayMode, opts ...rss.AppOption) error {
	return rss.RunApp(feeds, mode, opts...)
}
