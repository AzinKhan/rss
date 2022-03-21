package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/AzinKhan/rss"
	"github.com/gdamore/tcell/v2"
	"github.com/playwright-community/playwright-go"
	"github.com/rivo/tview"
)

const (
	feedsFile = ".rss/urls.txt"
)

var browser playwright.Browser = nil

func newLineWrapper(softLimit int) func(string) []string {
	return func(body string) []string {
		var result []string
		for len(body) > softLimit {
			delimiter := body[softLimit]
			lineBreakIdx := softLimit
			if string(delimiter) != " " {
				// Find the next space
				lineBreakIdx += findIndexWhere(body[lineBreakIdx:], " ")
			}
			line := body[:lineBreakIdx]
			result = append(result, line)
			body = body[lineBreakIdx:]
		}
		if len(body) > 0 {
			result = append(result, body)
		}
		return result

	}
}

func findIndexWhere(s string, target string) int {
	for i, char := range s {
		if string(char) != target {
			continue
		}
		return i
	}
	return 0
}

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

	feeds := rss.RefreshFeeds(urls)
	feedItems := rss.GetFeedItems(feeds, rss.OldestItem(maxAge), rss.Deduplicate(), itemFilter(maxItems))

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	defer pw.Stop()
	browser, err = pw.Firefox.Launch()
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	err = display(feedItems, displayMode)
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

func fetchText(url string, w io.Writer) {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	_, err = page.Goto(fmt.Sprintf("about:reader?url=%s", url))
	if err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	// Need to wait for the reader mode to take effect
	time.Sleep(1 * time.Second)

	entries, err := page.QuerySelectorAll("div[class='moz-reader-content reader-show-element']")
	if err != nil {
		log.Fatalf("could not get entries: %v", err)
	}

	wrapLines := newLineWrapper(72)
	for _, entry := range entries {
		titleElement, err := entry.QuerySelector("h3")
		if err != nil {
			fmt.Println(err)
			continue
		}
		if titleElement != nil {
			title, err := titleElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Fprintf(w, title)
		}
		bodyElements, err := entry.QuerySelectorAll("p")
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, bodyElement := range bodyElements {
			body, err := bodyElement.TextContent()
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, line := range wrapLines(body) {
				fmt.Fprintf(w, "\t%s\n", strings.TrimSpace(line))
			}
			fmt.Fprintf(w, "\n")
		}
	}

}

func display(feedItems []rss.FeedItem, mode rss.DisplayMode, opts ...rss.DisplayOption) error {
	app := tview.NewApplication()

	list := tview.NewList()
	list.ShowSecondaryText(false)

	textView := tview.NewTextView().SetDynamicColors(true)
	textView.SetChangedFunc(func() {
		app.Draw()
	})
	listFlex := tview.NewFlex()
	listFlex.AddItem(list, 0, 1, true)
	listFlex.SetBorder(true)

	textFlex := tview.NewFlex()
	textFlex.AddItem(textView, 0, 1, false)
	textFlex.SetBorder(true)

	listFlex.SetBorderColor(tcell.ColorGreen)
	textFlex.SetBorderColor(tcell.ColorGray)

	flex := tview.NewFlex()
	flex.AddItem(listFlex, 0, 1, true)
	flex.AddItem(textFlex, 0, 1, false)

	displayItems := mode(feedItems)
	for i, item := range displayItems {
		for _, o := range opts {
			item = o(item)
		}
		link := ""
		if len(item.Links) > 0 {
			link = item.Links[0]
		}
		list.InsertItem(i, item.Title, link, 0, nil)
	}
	list.AddItem("Quit", "Press to exit", 'q', func() {
		app.Stop()
	})

	toggleBorder := func(ps ...*tview.Box) {
		if listFlex.HasFocus() {
			listFlex.SetBorderColor(tcell.ColorGreen)
			textFlex.SetBorderColor(tcell.ColorGray)
			return
		}
		textFlex.SetBorderColor(tcell.ColorGreen)
		listFlex.SetBorderColor(tcell.ColorGray)
	}

	textView.SetDoneFunc(func(key tcell.Key) {
		app.SetFocus(list)
		toggleBorder()
	})

	list.SetHighlightFullLine(true)
	list.SetSelectedFunc(func(i int, main, secondary string, r rune) {
		if secondary == "" {
			return
		}
		textView.Clear()
		fmt.Fprintln(textView, secondary)
		fmt.Fprintf(textView, "\n")
		fetchText(secondary, textView)
		app.SetFocus(textView)
		textView.ScrollToBeginning()
		toggleBorder()
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ, tcell.KeyCtrlC:
			app.Stop()
		}
		return event
	})
	app.SetRoot(flex, true)
	err := app.Run()
	if err != nil {
		return err
	}

	return nil
}
