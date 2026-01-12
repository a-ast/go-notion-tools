package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"notion-tools/internal/notion"
)

const (
	NotionDatabaseID   = "6861df528fd14ac0934954d2e90fb015"
	NotionDataSourceID = "dc70f391-ee49-4e69-9aad-52c6ac9b16c0"

	defaultWhoPropName = "Who"
)

// ---- Main ----

func main() {
	var (
		tokenFlag = flag.String("token", "", "Notion integration token (or set NOTION_TOKEN)")
		fieldName = flag.String("field", defaultWhoPropName, "Property name to extract (default: who)")
		unique    = flag.Bool("unique", false, "Print unique values only (sorted)")
	)
	flag.Parse()

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("NOTION_TOKEN"))
	}
	if token == "" {
		fatal(errors.New("missing token: pass -token or set NOTION_TOKEN"))
	}

	field := strings.TrimSpace(*fieldName)
	if field == "" {
		fatal(errors.New("field name cannot be empty"))
	}

	ctx := context.Background()
	client := notion.NewClient(token)

	// Reduce payload to just the property we care about.
	qp := url.Values{}
	qp.Add("filter_properties[]", field)

	valuesSet := map[string]struct{}{}

	var cursor *string
	for {
		req := notion.QueryRequest{
			PageSize:    notion.DefaultPageSize,
			StartCursor: cursor,
		}

		var resp notion.QueryResponse
		if err := client.Do(ctx, http.MethodPost, "/data_sources/"+NotionDataSourceID+"/query", qp, req, &resp); err != nil {
			fatal(err)
		}

		for _, pg := range resp.Results {
			prop, ok := pg.Properties[field]
			if !ok {
				fatal(fmt.Errorf("property %q not found on returned pages; check the exact column name in Notion", field))
			}
			vals := notion.ExtractStrings(prop)
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				if *unique {
					valuesSet[v] = struct{}{}
				} else {
					fmt.Println(v)
				}
			}
		}

		if !resp.HasMore || resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	if *unique {
		out := make([]string, 0, len(valuesSet))
		for v := range valuesSet {
			out = append(out, v)
		}
		sort.Strings(out)
		for _, v := range out {
			fmt.Println(v)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
