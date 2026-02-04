package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
)

func PrintPages(pages []Page, asJSON bool) error {
	if asJSON {
		return printJSON(pages)
	}

	if len(pages) == 0 {
		fmt.Println("No pages found.")
		return nil
	}

	table := NewTable("ID", "TITLE", "LAST EDITED", "URL")
	for _, p := range pages {
		table.AddRow(
			TruncateID(p.ID),
			Truncate(p.Title, 50),
			formatTime(p.LastEditedTime),
			p.URL,
		)
	}
	table.Render()
	return nil
}

func PrintPage(page Page, asJSON bool) error {
	if asJSON {
		return printJSON(page)
	}

	titleStyle := color.New(color.Bold, color.FgWhite)
	labelStyle := color.New(color.Faint)

	if page.Icon != "" {
		titleStyle.Printf("%s ", page.Icon)
	}
	titleStyle.Println(page.Title)
	fmt.Println()

	labelStyle.Print("ID:           ")
	fmt.Println(page.ID)

	labelStyle.Print("URL:          ")
	fmt.Println(page.URL)

	labelStyle.Print("Created:      ")
	fmt.Println(page.CreatedTime.Format(time.RFC3339))

	labelStyle.Print("Last edited:  ")
	fmt.Println(page.LastEditedTime.Format(time.RFC3339))

	if page.Archived {
		labelStyle.Print("Status:       ")
		color.New(color.FgYellow).Println("Archived")
	}

	if page.Content != "" {
		fmt.Println()
		labelStyle.Println("─── Content ───")
		fmt.Println()

		if err := RenderMarkdown(page.Content); err != nil {
			fmt.Println(page.Content)
		}
	}

	return nil
}

func PrintDatabases(dbs []Database, asJSON bool) error {
	if asJSON {
		return printJSON(dbs)
	}

	if len(dbs) == 0 {
		fmt.Println("No databases found.")
		return nil
	}

	table := NewTable("ID", "TITLE", "DESCRIPTION", "URL")
	for _, db := range dbs {
		table.AddRow(
			TruncateID(db.ID),
			Truncate(db.Title, 40),
			Truncate(db.Description, 30),
			db.URL,
		)
	}
	table.Render()
	return nil
}

func PrintSearchResults(results []SearchResult, asJSON bool) error {
	if asJSON {
		return printJSON(results)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	table := NewTable("TYPE", "ID", "TITLE", "URL")
	for _, r := range results {
		typeStr := formatType(r.Type)
		table.AddRow(
			typeStr,
			TruncateID(r.ID),
			Truncate(r.Title, 50),
			r.URL,
		)
	}
	table.Render()
	return nil
}

func PrintComments(comments []Comment, asJSON bool) error {
	if asJSON {
		return printJSON(comments)
	}

	if len(comments) == 0 {
		fmt.Println("No comments found.")
		return nil
	}

	authorStyle := color.New(color.Bold)
	timeStyle := color.New(color.Faint)

	for i, c := range comments {
		if i > 0 {
			fmt.Println()
		}

		authorStyle.Print(c.CreatedBy)
		fmt.Print(" ")
		timeStyle.Println(formatTime(c.CreatedTime))
		fmt.Println(c.Content)
	}

	return nil
}

func PrintError(err error) {
	errStyle := color.New(color.FgRed, color.Bold)
	errStyle.Fprint(os.Stderr, "Error: ")
	fmt.Fprintln(os.Stderr, err.Error())
}

func PrintSuccess(message string) {
	successStyle := color.New(color.FgGreen)
	successStyle.Print("✓ ")
	fmt.Println(message)
}

func PrintWarning(message string) {
	warnStyle := color.New(color.FgYellow)
	warnStyle.Print("⚠ ")
	fmt.Println(message)
}

func PrintInfo(message string) {
	infoStyle := color.New(color.Faint)
	infoStyle.Println(message)
}

type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2 Jan 2006")
	}
}

func formatType(t string) string {
	switch t {
	case "page":
		return "📄 page"
	case "database":
		return "🗃️  db"
	default:
		return t
	}
}
