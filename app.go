package rss

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

func RunApp(feeds <-chan *Feed, mode DisplayMode, opts ...AppOption) error {
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
			if feed == nil {
				continue
			}
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
				list.InsertItem(i, formatFeedInteractive(item), link, 0, nil)
				i++
			}
			app.Draw()
			// Keep the cursor where it was
			list = list.SetCurrentItem(currentPosition)
		}
	}()

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

	var b *Browser
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		b, err = NewBrowser()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	}()

	list.SetSelectedFunc(func(i int, main, secondary string, r rune) {
		if secondary == "" {
			return
		}
		if b == nil {
			wg.Wait()
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
		case tcell.KeyRight:
			if app.GetFocus() != textView {
				app.SetFocus(textView)
				toggleBorder()
				return nil
			}
		case tcell.KeyLeft:
			if app.GetFocus() != listFlex {
				app.SetFocus(listFlex)
				toggleBorder()
				return nil
			}
		case tcell.KeyDown:
			focus := app.GetFocus()
			_, isList := focus.(*tview.List)
			if isList {
				n := list.GetItemCount() - 1
				currentItem := list.GetCurrentItem()
				if n == currentItem {
					// Swallow the event to stop jumping to the start
					return nil
				}
			}
		case tcell.KeyUp:
			focus := app.GetFocus()
			_, isList := focus.(*tview.List)
			if isList {
				currentItem := list.GetCurrentItem()
				if currentItem == 0 {
					// Swallow the event to stop jumping to the end
					return nil
				}
			}

		}
		return event
	})
	app.SetRoot(flex, true)
	return app.Run()
}
