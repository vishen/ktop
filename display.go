package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	termbox "github.com/nsf/termbox-go"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	leftPadding = 2
	topPadding  = 0
	minRowSize  = 10
)

var (
	termWidth          int
	termHeight         int
	filterString       string
	mouseX             int
	mouseY             int
	orderOption        OrderOption
	podMetrics         []PodMetrics
	selectedID         string
	selectedIndex      int = -1
	infoString         string
	updateLock         sync.Mutex
	previousPodMetrics = map[string]PodMetrics{}
)

type TermColor struct {
	bg termbox.Attribute
	fg termbox.Attribute
}

var (
	normalColor         = TermColor{bg: termbox.ColorBlack, fg: termbox.ColorWhite}
	headingColor        = TermColor{bg: termbox.ColorBlack, fg: termbox.ColorWhite | termbox.AttrBold}
	highlightedColor    = TermColor{bg: termbox.ColorWhite, fg: termbox.ColorBlack}
	changeIncreaseColor = TermColor{bg: termbox.ColorGreen, fg: termbox.ColorWhite | termbox.AttrBold}
	changeDecreaseColor = TermColor{bg: termbox.ColorRed, fg: termbox.ColorWhite | termbox.AttrBold}
	headerColor         = TermColor{bg: termbox.ColorWhite, fg: termbox.ColorBlack}
	footerColor         = TermColor{bg: termbox.ColorWhite, fg: termbox.ColorBlack}
)

type DisplayHeader struct {
	name           string
	getColumn      func(p PodMetrics) string
	maxLength      int
	forceMaxLength int
}

func (dh *DisplayHeader) GetName() string {
	return dh.getPossibleName(dh.name)
}

func (dh *DisplayHeader) getPossibleName(name string) string {
	if dh.forceMaxLength > 0 && len(name) > dh.forceMaxLength {
		return name[:dh.forceMaxLength/2] + "..." + name[len(name)-dh.forceMaxLength/2:]
	}
	return name
}

func (dh *DisplayHeader) GetLength() int {
	if dh.forceMaxLength > 0 {
		// TODO(vishen): WHY DO I NEED THIS?!!!!!
		return dh.forceMaxLength + 3
	}
	if dh.maxLength < minRowSize {
		return minRowSize
	}
	return dh.maxLength
}

func (dh *DisplayHeader) GetFrom(p PodMetrics) string {
	return dh.getPossibleName(dh.getColumn(p))
}

func (dh *DisplayHeader) Record(p PodMetrics) {
	value := dh.GetFrom(p)
	if len(value) > dh.maxLength {
		dh.maxLength = len(value)
	}
}

var (
	displayHeaders = []*DisplayHeader{
		{name: "NAMESPACE", getColumn: func(p PodMetrics) string { return p.Namespace }},
		{name: "POD", getColumn: func(p PodMetrics) string { return p.Pod }},
		{name: "CONTAINER", getColumn: func(p PodMetrics) string { return p.Container }},
		{name: "CPU", getColumn: func(p PodMetrics) string { return p.CPU }},
		{name: "MEM", getColumn: func(p PodMetrics) string { return p.MEM }},
	}
)

type OrderOption int

const (
	OrderNotSet OrderOption = iota
	OrderCPUAsc
	OrderCPUDec
	OrderMEMAsc
	OrderMEMDec
)

func setOrderOption(sortOrderOption OrderOption) {
	orderOption = sortOrderOption
}

func sortMetricsByOrder(podMetrics []PodMetrics) {
	order := 1
	fromUsage := func(pm PodMetrics) *resource.Quantity {
		return pm.Usage.Cpu()
	}

	switch orderOption {
	case OrderCPUDec:
		// Sort options are defaulted to this
	case OrderCPUAsc:
		order = -1
	case OrderMEMDec:
		fromUsage = func(pm PodMetrics) *resource.Quantity {
			return pm.Usage.Memory()
		}
	case OrderMEMAsc:
		order = -1
		fromUsage = func(pm PodMetrics) *resource.Quantity {
			return pm.Usage.Memory()
		}
	default:
		sort.Slice(podMetrics, func(i, j int) bool {
			pi := podMetrics[i]
			pj := podMetrics[j]
			if pi.Pod == pj.Pod {
				return pi.Container < pj.Container
			}
			return pi.Pod < pj.Pod
		})
		return
	}

	sort.Slice(podMetrics, func(i, j int) bool {
		pi := podMetrics[i]
		pj := podMetrics[j]
		result := fromUsage(pi).Cmp(*fromUsage(pj))
		if result == 0 {
			if pi.Pod == pj.Pod {
				return pi.Container < pj.Container
			}
			return pi.Pod < pj.Pod
		}
		return result == order
	})
}

func setMouseClick(x, y int, key termbox.Key) {
	// TODO: Remove this lock
	updateLock.Lock()
	defer updateLock.Unlock()
	switch key {
	case termbox.MouseLeft:
		if len(podMetrics) >= y-2 && y > 1 {
			selectedIndex = y - 2
			selectedID = podMetrics[selectedIndex].UniqueID()
		}
	case termbox.MouseRight:
		selectedID = ""
	}
}

func updateSelectedID(i int) {
	// TODO: Remove this lock
	updateLock.Lock()
	defer updateLock.Unlock()

	selectedIndex += i
	if selectedIndex < 0 {
		selectedIndex = 0
	} else if selectedIndex >= len(podMetrics) {
		selectedIndex = len(podMetrics) - 1
	}
	selectedID = podMetrics[selectedIndex].UniqueID()
}

func snapshot() {
	// If we are toggling disable snapshot
	if len(previousPodMetrics) > 0 {
		previousPodMetrics = map[string]PodMetrics{}
		return
	}
	// Make a copy so we can tell what has changed
	for _, pm := range podMetrics {
		previousPodMetrics[pm.UniqueID()] = pm
	}
}

func updateScreen() {
	// TODO: Remove this lock
	updateLock.Lock()
	defer updateLock.Unlock()
	termbox.Clear(termbox.ColorBlack, termbox.ColorBlack)

	headerString := fmt.Sprintf("filter: %s", filterString)
	outputWord(headerString, 0, 0, headerColor)

	// TODO: Cache these values, otherwise we get noticable lag when typing
	// as there is lock competition; this should ideally happen in the background.
	// This shouldn't use a lock if possible.
	allPodMetrics := kubeMetrics.GetMetrics()

	podMetrics = make([]PodMetrics, 0, len(allPodMetrics))
	for _, pr := range allPodMetrics {
		// Filter out any pods based on the filter string
		valid := false
		if filterString != "" {
			names := []string{
				pr.Namespace,
				pr.Pod,
				pr.Container,
			}
			for _, n := range names {
				if strings.Contains(n, filterString) {
					valid = true
					break
				}
			}
		} else {
			valid = true
		}
		if !valid {
			continue
		}

		podMetrics = append(podMetrics, pr)

		// Record the longest string so we can display column lengths
		// correctly
		for _, header := range displayHeaders {
			header.Record(pr)
		}
	}

	// sort metrics
	sortMetricsByOrder(podMetrics)

	// Total column width
	totalHeaderWidth := 0
	for _, header := range displayHeaders {
		// Add a single space between each header, can't think of
		// a better place to put this.
		totalHeaderWidth += header.GetLength() + 1
	}

	/*if totalHeaderWidth > termWidth {
		possibleHeaderWidth := termWidth / len(displayHeaders)
		for _, header := range displayHeaders {
			if header.GetLength() > possibleHeaderWidth {
				header.forceMaxLength = possibleHeaderWidth
			}
		}
	}*/

	{
		// Display headers
		currentX := 0
		for _, header := range displayHeaders {
			// TODO(vishen): FIX THIS HACK!
			// TODO(vishen): FIX THIS HACK!
			// TODO(vishen): FIX THIS HACK!
			// TODO(vishen): FIX THIS HACK!
			// TODO(vishen): FIX THIS HACK!
			if totalHeaderWidth > termWidth {
				if header.name == "POD" {
					header.forceMaxLength = 25
				} else if header.name == "CONTAINER" {
					header.forceMaxLength = 20
				} else if header.name == "CPU" || header.name == "MEM" {
					header.forceMaxLength = 5
				}
			}
			outputWord(header.GetName(), currentX, 1, headingColor)
			// Add a single space between each header, can't think of
			// a better place to put this.
			currentX += header.GetLength() + 1
		}
	}

	for y, pr := range podMetrics {
		// Don't let the data go over the footer
		if y > termHeight-3 {
			break
		}

		currentX := 0
		for _, header := range displayHeaders {
			color := normalColor
			value := header.GetFrom(pr)
			// TODO(vishen): super hacky, but will work for now
			switch header.name {
			case "CPU":
				if p, ok := previousPodMetrics[pr.UniqueID()]; ok {
					switch pr.Usage.Cpu().Cmp(*p.Usage.Cpu()) {
					case 1:
						color = changeIncreaseColor
						value = fmt.Sprintf("%s^%s", p.CPU, value)
					case -1:
						color = changeDecreaseColor
						value = fmt.Sprintf("%sv%s", p.CPU, value)
					default:
						color = normalColor
					}
				}
			case "MEM":
				if p, ok := previousPodMetrics[pr.UniqueID()]; ok {
					switch pr.Usage.Memory().Cmp(*p.Usage.Memory()) {
					case 1:
						color = changeIncreaseColor
						value = fmt.Sprintf("%s^%s", p.MEM, value)
					case -1:
						color = changeDecreaseColor
						value = fmt.Sprintf("%sv%s", p.MEM, value)
					default:
						color = normalColor
					}
				}
			}
			if pr.UniqueID() == selectedID {
				color = highlightedColor
				selectedIndex = y
				infoString = pr.InfoString()
			}
			outputWord(value, currentX, y+2, color)
			currentX += header.GetLength() + 1
		}
	}

	outputWord(infoString, 0, termHeight-3, footerColor)

	// Draw footer with options
	footerString := "Sort by (1) CPU Dec / (2) CPU Asc / (3) Mem Dec / (4) Mem Asc | (SPACE) Snapshot | (ESC) Quit"
	if len(previousPodMetrics) > 0 {
		footerString += " -- Snapshot taken!"
	}
	outputWord(footerString, 0, termHeight-2, footerColor)

	termbox.Flush()

}

func getX(x int) int {
	return x + leftPadding
}

func getY(y int) int {
	return y + topPadding
}

func outputWord(word string, startingX, y int, color TermColor) {
	startingX = getX(startingX)
	y = getY(y)
	for x, c := range word {
		termbox.SetCell(startingX+x, y, c, color.fg, color.bg)
	}
}
