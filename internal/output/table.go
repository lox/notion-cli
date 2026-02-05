package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"golang.org/x/term"
)

type Table struct {
	headers []string
	rows    [][]string
	out     io.Writer
}

func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
		out:     os.Stdout,
	}
}

func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

func (t *Table) Render() {
	if len(t.rows) == 0 {
		return
	}

	widths := t.calculateWidths()
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	headerStyle := color.New(color.Bold)
	dimStyle := color.New(color.Faint)

	if isTTY {
		t.printRow(t.headers, widths, headerStyle)
		t.printSeparator(widths, dimStyle)
	}

	for _, row := range t.rows {
		t.printRow(row, widths, nil)
	}
}

func (t *Table) calculateWidths() []int {
	widths := make([]int, len(t.headers))

	for i, h := range t.headers {
		widths[i] = utf8.RuneCountInString(h)
	}

	for _, row := range t.rows {
		for i, col := range row {
			if i < len(widths) {
				w := utf8.RuneCountInString(col)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	maxWidth := t.terminalWidth() - (len(widths) * 2)
	if maxWidth < 40 {
		maxWidth = 80
	}

	total := 0
	for _, w := range widths {
		total += w
	}

	if total > maxWidth {
		scale := float64(maxWidth) / float64(total)
		for i := range widths {
			widths[i] = int(float64(widths[i]) * scale)
			if widths[i] < 4 {
				widths[i] = 4
			}
		}
	}

	return widths
}

func (t *Table) terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 120
	}
	return width
}

func (t *Table) printRow(row []string, widths []int, style *color.Color) {
	var parts []string
	for i, col := range row {
		if i >= len(widths) {
			break
		}
		truncated := Truncate(col, widths[i])
		padded := fmt.Sprintf("%-*s", widths[i], truncated)
		parts = append(parts, padded)
	}

	line := strings.Join(parts, "  ")
	if style != nil {
		_, _ = style.Fprintln(t.out, line)
	} else {
		_, _ = fmt.Fprintln(t.out, line)
	}
}

func (t *Table) printSeparator(widths []int, style *color.Color) {
	var parts []string
	for _, w := range widths {
		parts = append(parts, strings.Repeat("─", w))
	}
	line := strings.Join(parts, "──")
	if style != nil {
		_, _ = style.Fprintln(t.out, line)
	} else {
		_, _ = fmt.Fprintln(t.out, line)
	}
}

func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func TruncateID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
