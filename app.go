package rss

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	app   *tview.Application
	b     *Browser
	feeds chan *Feed
}

type appOptions struct {
	display []DisplayOption
	filters []Filter
}

type AppOption func(*appOptions)

func WithDisplayOptions(opts ...DisplayOption) AppOption {
	return func(ao *appOptions) {
		ao.display = append(ao.display, opts...)
	}
}

func WithFilters(filters ...Filter) AppOption {
	return func(ao *appOptions) {
		ao.filters = append(ao.filters, filters...)
	}
}

func NewApp(feeds chan *Feed, b *Browser, mode DisplayMode, opts ...AppOption) *App {
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

	options := &appOptions{}

	for _, o := range opts {
		o(options)
	}

	go func() {
		var i int
		for feed := range feeds {
			currentPosition := list.GetCurrentItem()
			feedItems := UnpackFeed(feed, options.filters...)
			items := make([]FeedItem, 0, len(feedItems))
			for _, item := range mode(feedItems) {
				for _, o := range options.display {
					item = o(item)
				}
				items = append(items, item)
			}

			for _, item := range items {
				link := ""
				if len(item.Links) > 0 {
					link = item.Links[0]
				}
				list.InsertItem(i, item.Title, link, 0, nil)
				i++
			}
			app.Draw()
			// Keep the cursor where it was
			list = list.SetCurrentItem(currentPosition)
		}
	}()

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
		page, err := b.NewPage(secondary)
		if err != nil {
			fmt.Fprintf(textView, err.Error())
			return
		}
		io.Copy(textView, page)
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
	return &App{
		app:   app,
		b:     b,
		feeds: feeds,
	}
}

func (a *App) Run() error {
	return a.app.Run()
}
