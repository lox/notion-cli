package mcp

import "time"

// Notion domain types

type Page struct {
	ID             string         `json:"id"`
	Object         string         `json:"object"`
	CreatedTime    time.Time      `json:"created_time"`
	LastEditedTime time.Time      `json:"last_edited_time"`
	CreatedBy      UserRef        `json:"created_by"`
	LastEditedBy   UserRef        `json:"last_edited_by"`
	Archived       bool           `json:"archived"`
	Icon           *Icon          `json:"icon,omitempty"`
	Cover          *Cover         `json:"cover,omitempty"`
	Properties     map[string]any `json:"properties"`
	Parent         Parent         `json:"parent"`
	URL            string         `json:"url"`
	PublicURL      string         `json:"public_url,omitempty"`
}

type Database struct {
	ID             string         `json:"id"`
	Object         string         `json:"object"`
	CreatedTime    time.Time      `json:"created_time"`
	LastEditedTime time.Time      `json:"last_edited_time"`
	CreatedBy      UserRef        `json:"created_by"`
	LastEditedBy   UserRef        `json:"last_edited_by"`
	Title          []RichText     `json:"title"`
	Description    []RichText     `json:"description"`
	Icon           *Icon          `json:"icon,omitempty"`
	Cover          *Cover         `json:"cover,omitempty"`
	Properties     map[string]any `json:"properties"`
	Parent         Parent         `json:"parent"`
	URL            string         `json:"url"`
	Archived       bool           `json:"archived"`
	IsInline       bool           `json:"is_inline"`
	PublicURL      string         `json:"public_url,omitempty"`
}

type Block struct {
	ID             string    `json:"id"`
	Object         string    `json:"object"`
	Type           string    `json:"type"`
	CreatedTime    time.Time `json:"created_time"`
	LastEditedTime time.Time `json:"last_edited_time"`
	CreatedBy      UserRef   `json:"created_by"`
	LastEditedBy   UserRef   `json:"last_edited_by"`
	HasChildren    bool      `json:"has_children"`
	Archived       bool      `json:"archived"`
	Parent         Parent    `json:"parent"`
}

type User struct {
	ID        string  `json:"id"`
	Object    string  `json:"object"`
	Type      string  `json:"type"`
	Name      string  `json:"name,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Person    *Person `json:"person,omitempty"`
	Bot       *Bot    `json:"bot,omitempty"`
}

type UserRef struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type Person struct {
	Email string `json:"email"`
}

type Bot struct {
	Owner         BotOwner `json:"owner"`
	WorkspaceName string   `json:"workspace_name,omitempty"`
}

type BotOwner struct {
	Type      string `json:"type"`
	Workspace bool   `json:"workspace,omitempty"`
	User      *User  `json:"user,omitempty"`
}

type Comment struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	Parent         Parent     `json:"parent"`
	DiscussionID   string     `json:"discussion_id"`
	CreatedTime    time.Time  `json:"created_time"`
	LastEditedTime time.Time  `json:"last_edited_time"`
	CreatedBy      UserRef    `json:"created_by"`
	RichText       []RichText `json:"rich_text"`
}

type Parent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
	BlockID    string `json:"block_id,omitempty"`
	Workspace  bool   `json:"workspace,omitempty"`
}

type RichText struct {
	Type        string       `json:"type"`
	PlainText   string       `json:"plain_text"`
	Href        string       `json:"href,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Text        *TextContent `json:"text,omitempty"`
	Mention     *Mention     `json:"mention,omitempty"`
	Equation    *Equation    `json:"equation,omitempty"`
}

type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

type Link struct {
	URL string `json:"url"`
}

type Mention struct {
	Type string `json:"type"`
}

type Equation struct {
	Expression string `json:"expression"`
}

type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

type Icon struct {
	Type     string `json:"type"`
	Emoji    string `json:"emoji,omitempty"`
	External *File  `json:"external,omitempty"`
	File     *File  `json:"file,omitempty"`
}

type Cover struct {
	Type     string `json:"type"`
	External *File  `json:"external,omitempty"`
	File     *File  `json:"file,omitempty"`
}

type File struct {
	URL        string    `json:"url"`
	ExpiryTime time.Time `json:"expiry_time,omitempty"`
}

// Search types

type SearchResult struct {
	Object     string `json:"object"`
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
	Type       string `json:"type,omitempty"`
}

type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	NextCursor string         `json:"next_cursor,omitempty"`
	HasMore    bool           `json:"has_more"`
}

// Response wrappers

type UsersResponse struct {
	Users      []User `json:"users"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type CommentsResponse struct {
	Comments   []Comment `json:"comments"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}
