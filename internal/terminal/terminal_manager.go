package terminal

import (
	"fmt"
	"strings"
)

const (
	// ANSI escape codes
	escape         = "\x1b"
	cursorNextLine = escape + "[1E"
	cursorPrevLine = escape + "[1F"
	cursorToCol    = escape + "[%dG" // 1 based
	clearLine      = escape + "[2K"
	cursorHide     = escape + "[?25l"
	cursorShow     = escape + "[?25h"
)

type BufferedArea struct {
	last          []string
	cursor_hidden bool
}

func (ba *BufferedArea) Update(lines []string) {

	if !ba.cursor_hidden {
		fmt.Print(cursorHide)
		ba.cursor_hidden = true
	}

	if ba.last == nil {
		ba.last = lines
		for _, l := range lines {
			fmt.Println(l)
		}
		return
	}

	for range len(ba.last) {
		fmt.Print(cursorPrevLine)
	}

	for i, l := range lines {
		if i >= len(ba.last) {
			fmt.Println(l)
			continue
		}

		if l == ba.last[i] {
			fmt.Printf(cursorNextLine)
			continue
		}

		for j, c := range l {
			if j < len(ba.last[i]) && rune(ba.last[i][j]) == c {
				continue
			}
			fmt.Printf(cursorToCol, j+1)
			fmt.Printf("%c", c)
		}
		fmt.Printf(cursorNextLine)
	}

	if len(lines) < len(ba.last) {
		for range len(ba.last) - len(lines) {
			fmt.Println(clearLine)
		}
	}

	ba.last = lines
}

func (ba *BufferedArea) Close() {
	fmt.Print(cursorShow)
}

func ProgressBar(current, max, line_length int, suffix string) (string, error) {
	if current < 0 {
		return "", fmt.Errorf("current can not be less than 0")
	}
	if max < 0 {
		return "", fmt.Errorf("max can not be less than 0")
	}
	if current > max {
		return "", fmt.Errorf("current must be less than or equal to max")
	}

	required_suffix := len(suffix)
	if required_suffix != 0 {
		required_suffix++ // space gap
	}
	available := line_length - required_suffix
	if available < 3 { // '[ ]' is the smallest bar
		return "", fmt.Errorf("line_length is not sufficient to present the progress bar")
	}

	segments := available - 2
	progress_per_segment := max / segments
	current_progress := current / progress_per_segment

	var line strings.Builder
	line.WriteString("[")

	for range current_progress {
		line.WriteString("=")
	}
	for range segments - current_progress {
		line.WriteString(" ")
	}
	line.WriteString("]")

	if len(suffix) > 0 {
		line.WriteString(" " + suffix)
	}

	return line.String(), nil
}
