package output

import "time"

type Page struct {
	ID             string
	Title          string
	URL            string
	CreatedTime    time.Time
	LastEditedTime time.Time
	ParentType     string
	ParentID       string
	Archived       bool
	Icon           string
	Content        string
}

type Database struct {
	ID             string
	Title          string
	URL            string
	CreatedTime    time.Time
	LastEditedTime time.Time
	Description    string
	Icon           string
}

type SearchResult struct {
	ID         string
	Type       string
	Title      string
	URL        string
	ParentType string
	ParentID   string
}

type Comment struct {
	ID             string
	DiscussionID   string
	Context        string
	Resolved       bool
	CreatedTime    time.Time
	LastEditedTime time.Time
	CreatedBy      string
	CreatedByName  string
	Content        string
}
