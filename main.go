package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"notion-tools/internal/notion"
)

const (
	NotionChroniclesDataSourceID = "dc70f391-ee49-4e69-9aad-52c6ac9b16c0"
	NotionPeopleDatabaseID       = "2e7e1d14-ea06-80f8-8635-000bc244940f"

	defaultWhoPropName = "Who"
)

// ---- Main ----

func main() {
	var (
		tokenFlag = flag.String("token", "", "Notion integration token (or set NOTION_TOKEN)")
		fieldName = flag.String("field", defaultWhoPropName, "Property name to extract (default: who)")
	)
	flag.Parse()

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("NOTION_TOKEN"))
	}
	if token == "" {
		fatal(errors.New("missing token: pass -token or set NOTION_TOKEN"))
	}

	srcField := strings.TrimSpace(*fieldName)
	if srcField == "" {
		fatal(errors.New("field name cannot be empty"))
	}

	ctx := context.Background()
	client := notion.NewClient(token)

	// Reduce payload to just the property we care about.
	qp := url.Values{}
	qp.Add("filter_properties[]", "Name")
	qp.Add("filter_properties[]", srcField)

	var cursor *string
	for {
		req := notion.QueryRequest{
			PageSize:    notion.DefaultPageSize,
			StartCursor: cursor,
		}

		var resp notion.QueryResponse
		if err := client.Do(ctx, http.MethodPost, "/data_sources/"+NotionChroniclesDataSourceID+"/query", qp, req, &resp); err != nil {
			fatal(err)
		}

		for _, pg := range resp.Results {
			prop, ok := pg.Properties[srcField]
			title, _ := pg.Properties["Name"]
			fmt.Println(notion.ExtractString(title))

			if !ok {
				fatal(fmt.Errorf("property %q not found on returned pages; check the exact column name in Notion", srcField))
			}
			who := notion.ExtractString(prop)

			cleanedPersons := extractPersons(who)

			// Create/update people pages and collect their IDs
			var peoplePageIDs []string
			for _, personName := range cleanedPersons {
				if personName == "" {
					continue
				}

				// Check if a page with this name already exists
				existingPage, err := client.FindPageByTitle(ctx, NotionPeopleDatabaseID, personName)
				if err != nil {
					fatal(fmt.Errorf("failed to check for existing people page for %s: %w", personName, err))
				}

				var pageID string
				if existingPage != nil {
					// Page already exists, use its ID
					pageID = existingPage.ID
					fmt.Printf("Found existing page for %s: %s\n", personName, pageID)
				} else {
					// Create a new page in the people database
					peopleProps := map[string]notion.PropertyValue{
						"Name": {
							Type: "title",
							Title: []notion.RichText{
								{
									Type: "text",
									Text: &notion.TextContent{Content: personName},
								},
							},
						},
					}

					peoplePage, err := client.CreatePage(ctx, NotionPeopleDatabaseID, peopleProps)
					if err != nil {
						fatal(fmt.Errorf("failed to create people page for %s: %w", personName, err))
					}
					pageID = peoplePage.ID
					fmt.Printf("Created new page for %s: %s\n", personName, pageID)
				}
				peoplePageIDs = append(peoplePageIDs, pageID)
			}

			// Update the People field with the extracted persons
			if len(peoplePageIDs) == 0 {
				continue
			}

			relationRefs := make([]notion.RelationRef, 0, len(peoplePageIDs))
			for _, pageID := range peoplePageIDs {
				relationRefs = append(relationRefs, notion.RelationRef{ID: pageID})
			}

			updateProps := map[string]notion.PropertyValue{
				"People": {
					Type:     "relation",
					Relation: relationRefs,
				},
			}

			if err := client.UpdatePage(ctx, pg.ID, updateProps); err != nil {
				fatal(fmt.Errorf("failed to update page %s: %w", pg.ID, err))
			}

		}

		if !resp.HasMore || resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
}

func extractPersons(who string) []string {
	persons := strings.Split(who, ", ")
	var cleanedPersons []string
	for _, p := range persons {
		p = strings.TrimSpace(p)
		cleanedPersons = append(cleanedPersons, p)
	}
	return cleanedPersons
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
