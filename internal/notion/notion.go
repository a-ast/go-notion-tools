package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// BaseURL is the base URL for Notion API
	BaseURL = "https://api.notion.com/v1"
	// NotionVersion is the API version
	NotionVersion = "2025-09-03"
	// DefaultPageSize is the default page size for queries
	DefaultPageSize = 100
	// HTTPTimeout is the timeout for HTTP requests
	HTTPTimeout = 30 * time.Second
)

// Client represents a Notion API client
type Client struct {
	token string
	http  *http.Client
}

// NewClient creates a new Notion API client
func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: HTTPTimeout},
	}
}

// Do performs an HTTP request to the Notion API
func (c *Client) Do(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	u := c.url(path, q)

	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", NotionVersion)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "notion-tools/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notion API %s %s failed: status=%d body=%s",
			method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("unmarshal response: %w (body=%s)", err, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (*Client) url(path string, q url.Values) string {
	u := BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// QueryRequest represents a query request
type QueryRequest struct {
	PageSize    int     `json:"page_size,omitempty"`
	StartCursor *string `json:"start_cursor,omitempty"`
}

// QueryResponse represents a query response
type QueryResponse struct {
	Object     string  `json:"object"`
	Results    []Page  `json:"results"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}

// Page represents a Notion page
type Page struct {
	Object     string                   `json:"object"`
	ID         string                   `json:"id"`
	Properties map[string]PropertyValue `json:"properties"`
}

// PropertyValue represents a property value
type PropertyValue struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Title       []RichText     `json:"title,omitempty"`
	RichText    []RichText     `json:"rich_text,omitempty"`
	Select      *SelectOption  `json:"select,omitempty"`
	MultiSelect []SelectOption `json:"multi_select,omitempty"`
	Status      *SelectOption  `json:"status,omitempty"`
	People      []User         `json:"people,omitempty"`
	Email       *string        `json:"email,omitempty"`
	URL         *string        `json:"url,omitempty"`
	PhoneNumber *string        `json:"phone_number,omitempty"`
	Number      *float64       `json:"number,omitempty"`
	Checkbox    *bool          `json:"checkbox,omitempty"`
	Date        *DateValue     `json:"date,omitempty"`
	Relation    []RelationRef  `json:"relation,omitempty"`
	Formula     *FormulaValue  `json:"formula,omitempty"`
	Rollup      *RollupValue   `json:"rollup,omitempty"`
}

// RichText represents rich text
type RichText struct {
	PlainText string `json:"plain_text"`
}

// SelectOption represents a select option
type SelectOption struct {
	Name string `json:"name"`
}

// User represents a user
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DateValue represents a date value
type DateValue struct {
	Start string  `json:"start"`
	End   *string `json:"end"`
}

// RelationRef represents a relation reference
type RelationRef struct {
	ID string `json:"id"`
}

// FormulaValue represents a formula value
type FormulaValue struct {
	Type    string     `json:"type"`
	String  *string    `json:"string,omitempty"`
	Number  *float64   `json:"number,omitempty"`
	Boolean *bool      `json:"boolean,omitempty"`
	Date    *DateValue `json:"date,omitempty"`
}

// RollupValue represents a rollup value
type RollupValue struct {
	Type   string          `json:"type"`
	Number *float64        `json:"number,omitempty"`
	Date   *DateValue      `json:"date,omitempty"`
	Array  []PropertyValue `json:"array,omitempty"`
}

// ExtractStrings extracts string values from a property value
func ExtractStrings(p PropertyValue) []string {
	switch p.Type {
	case "title":
		s := concatRichText(p.Title)
		if s == "" {
			return nil
		}
		return []string{s}

	case "rich_text":
		s := concatRichText(p.RichText)
		if s == "" {
			return nil
		}
		return []string{s}

	case "select":
		if p.Select == nil || p.Select.Name == "" {
			return nil
		}
		return []string{p.Select.Name}

	case "status":
		if p.Status == nil || p.Status.Name == "" {
			return nil
		}
		return []string{p.Status.Name}

	case "multi_select":
		if len(p.MultiSelect) == 0 {
			return nil
		}
		out := make([]string, 0, len(p.MultiSelect))
		for _, o := range p.MultiSelect {
			if o.Name != "" {
				out = append(out, o.Name)
			}
		}
		return out

	case "people":
		if len(p.People) == 0 {
			return nil
		}
		out := make([]string, 0, len(p.People))
		for _, u := range p.People {
			if u.Name != "" {
				out = append(out, u.Name)
			} else if u.ID != "" {
				out = append(out, u.ID)
			}
		}
		return out

	case "email":
		if p.Email == nil || *p.Email == "" {
			return nil
		}
		return []string{*p.Email}

	case "url":
		if p.URL == nil || *p.URL == "" {
			return nil
		}
		return []string{*p.URL}

	case "phone_number":
		if p.PhoneNumber == nil || *p.PhoneNumber == "" {
			return nil
		}
		return []string{*p.PhoneNumber}

	case "number":
		if p.Number == nil {
			return nil
		}
		return []string{strconv.FormatFloat(*p.Number, 'f', -1, 64)}

	case "checkbox":
		if p.Checkbox == nil {
			return nil
		}
		return []string{strconv.FormatBool(*p.Checkbox)}

	case "date":
		if p.Date == nil || p.Date.Start == "" {
			return nil
		}
		if p.Date.End != nil && *p.Date.End != "" {
			return []string{p.Date.Start + " → " + *p.Date.End}
		}
		return []string{p.Date.Start}

	case "relation":
		if len(p.Relation) == 0 {
			return nil
		}
		out := make([]string, 0, len(p.Relation))
		for _, r := range p.Relation {
			if r.ID != "" {
				out = append(out, r.ID)
			}
		}
		return out

	case "formula":
		if p.Formula == nil {
			return nil
		}
		switch p.Formula.Type {
		case "string":
			if p.Formula.String == nil || *p.Formula.String == "" {
				return nil
			}
			return []string{*p.Formula.String}
		case "number":
			if p.Formula.Number == nil {
				return nil
			}
			return []string{strconv.FormatFloat(*p.Formula.Number, 'f', -1, 64)}
		case "boolean":
			if p.Formula.Boolean == nil {
				return nil
			}
			return []string{strconv.FormatBool(*p.Formula.Boolean)}
		case "date":
			if p.Formula.Date == nil || p.Formula.Date.Start == "" {
				return nil
			}
			if p.Formula.Date.End != nil && *p.Formula.Date.End != "" {
				return []string{p.Formula.Date.Start + " → " + *p.Formula.Date.End}
			}
			return []string{p.Formula.Date.Start}
		default:
			return nil
		}

	case "rollup":
		if p.Rollup == nil {
			return nil
		}
		switch p.Rollup.Type {
		case "number":
			if p.Rollup.Number == nil {
				return nil
			}
			return []string{strconv.FormatFloat(*p.Rollup.Number, 'f', -1, 64)}
		case "date":
			if p.Rollup.Date == nil || p.Rollup.Date.Start == "" {
				return nil
			}
			if p.Rollup.Date.End != nil && *p.Rollup.Date.End != "" {
				return []string{p.Rollup.Date.Start + " → " + *p.Rollup.Date.End}
			}
			return []string{p.Rollup.Date.Start}
		case "array":
			var out []string
			for _, item := range p.Rollup.Array {
				out = append(out, ExtractStrings(item)...)
			}
			return out
		default:
			return nil
		}

	default:
		return nil
	}
}

func concatRichText(rts []RichText) string {
	var b strings.Builder
	for _, rt := range rts {
		b.WriteString(rt.PlainText)
	}
	return strings.TrimSpace(b.String())
}
