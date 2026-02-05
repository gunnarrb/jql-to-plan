package jira

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/andygrunwald/go-jira/v2/onpremise"
)

type Client struct {
	cloudClient           *cloud.Client
	onpremiseClient       *onpremise.Client
	isCloud               bool
	effortCustomFieldID   string
	epicLinkCustomFieldID string
}

type Ticket struct {
	Key            string
	Summary        string
	Link           string
	Assignee       string
	Status         string
	EffortDays     float64  // Effort in days from custom field cf[10105]
	EpicLink       string   // Key of the Epic this ticket belongs to
	DependencyKeys []string // Keys of tickets this ticket depends on
}

// NewClient creates a new Jira client.
// We'll simplisticly assume PAT implies simple Bearer auth or similar.
// go-jira v2 splits cloud and onpremise.
// For widely compatible PAT usage, we often just need a standard HTTP client that adds the header.
func NewClient(endpoint, pat, effortCustomFieldID, epicLinkCustomFieldID string) (*Client, error) {
	// Create a transport that adds the auth header
	tp := &patTransport{
		PAT:  pat,
		Base: http.DefaultTransport,
	}
	httpClient := &http.Client{Transport: tp}

	// Simple heuristic: if endpoint contains "atlassian.net", it's likely cloud, but
	// for a generic CLI handling "JIRA_ENDPOINT", onpremise logic is often safer for custom domains
	// unless we know it's cloud.
	// Actually, let's try to use the onpremise client which is often more generic for
	// simple JQL fetching if the API matches.
	// However, v2 forces a choice. Let's try onpremise first as it's the more "generic" enterprise case usually associated with PATs on self-hosted.
	// If the user is on Cloud, they usually use proper tokens + email.
	// BUT, the prompt says "JIRA_PAT", which implies Personal Access Token.
	// Let's assume onpremise API compatibility for simplicity, or we can try to detect.
	// For now, I will use `onpremise` client from the library which typically maps to the /rest/api/2 standard.

	client, err := onpremise.NewClient(endpoint, httpClient)
	if err != nil {
		return nil, err
	}

	if effortCustomFieldID != "" && !strings.HasPrefix(effortCustomFieldID, "customfield_") {
		effortCustomFieldID = "customfield_" + effortCustomFieldID
	}
	if epicLinkCustomFieldID != "" && !strings.HasPrefix(epicLinkCustomFieldID, "customfield_") {
		epicLinkCustomFieldID = "customfield_" + epicLinkCustomFieldID
	}

	return &Client{
		onpremiseClient:       client,
		effortCustomFieldID:   effortCustomFieldID,
		epicLinkCustomFieldID: epicLinkCustomFieldID,
	}, nil
}

type patTransport struct {
	PAT  string
	Base http.RoundTripper
}

func (t *patTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.PAT)
	return t.Base.RoundTrip(req)
}

func (c *Client) GetTickets(ctx context.Context, jql string) ([]Ticket, map[string]Ticket, error) {
	if c.onpremiseClient == nil {
		return nil, nil, fmt.Errorf("client not initialized")
	}

	// Search implementation - include custom field for effort and issuelinks
	fields := []string{"summary", "assignee", "status", "issuelinks", c.effortCustomFieldID}
	if c.epicLinkCustomFieldID != "" {
		fields = append(fields, c.epicLinkCustomFieldID)
	}

	issues, _, err := c.onpremiseClient.Issue.Search(ctx, jql, &onpremise.SearchOptions{
		Fields:     fields,
		StartAt:    0,
		MaxResults: 1000, // Fetch up to 1000 for now
	})
	if err != nil {
		return nil, nil, err
	}

	var tickets []Ticket
	for _, i := range issues {
		var assignee string
		if i.Fields.Assignee != nil {
			assignee = i.Fields.Assignee.DisplayName
			if assignee == "" {
				assignee = i.Fields.Assignee.Name
			}
		}

		// Extract effort from custom field
		effortDays, found := extractEffortDays(i.Fields.Unknowns, c.effortCustomFieldID)
		if !found || effortDays == 0 {
			fmt.Printf("Warning: Ticket %s: %s has missing or 0 effort\n", i.Key, i.Fields.Summary)
		}

		// Extract Epic Link
		var epicLink string
		if c.epicLinkCustomFieldID != "" {
			if val, ok := i.Fields.Unknowns[c.epicLinkCustomFieldID]; ok && val != nil {
				if strVal, ok := val.(string); ok {
					epicLink = strVal
				}
			}
		}

		var dependencyKeys []string
		for _, link := range i.Fields.IssueLinks {
			if link.Type.Name == "Dependent" && link.OutwardIssue != nil {
				dependencyKeys = append(dependencyKeys, link.OutwardIssue.Key)
			}
		}

		tickets = append(tickets, Ticket{
			Key:            i.Key,
			Summary:        i.Fields.Summary,
			Link:           i.Self,
			Assignee:       assignee,
			Status:         i.Fields.Status.Name,
			EffortDays:     effortDays,
			EpicLink:       epicLink,
			DependencyKeys: dependencyKeys,
		})
	}

	// Fetch Epic details if any epics were found
	epicMap := make(map[string]Ticket)
	if c.epicLinkCustomFieldID != "" {
		uniqueEpics := make(map[string]bool)
		for _, t := range tickets {
			if t.EpicLink != "" {
				uniqueEpics[t.EpicLink] = true
			}
		}

		if len(uniqueEpics) > 0 {
			var epicKeys []string
			for key := range uniqueEpics {
				epicKeys = append(epicKeys, key)
			}

			// Batch fetch epics in chunks of 50 (Jira limit is often around 50-100 for IN clause)
			// For simplicity in this CLI, we'll just try to fetch all if < 50, or chunk if needed.
			// Let's implement a simple chunking.
			chunkSize := 50
			for i := 0; i < len(epicKeys); i += chunkSize {
				end := i + chunkSize
				if end > len(epicKeys) {
					end = len(epicKeys)
				}
				batchKeys := epicKeys[i:end]

				jql := fmt.Sprintf("key in (%s)", strings.Join(batchKeys, ","))
				// We don't need the custom fields for the Epic itself, just Summary/Status
				epics, _, err := c.onpremiseClient.Issue.Search(ctx, jql, &onpremise.SearchOptions{
					Fields:     []string{"summary", "status", "issuelinks"}, // Include issuelinks if we ever need dependencies of epics
					StartAt:    0,
					MaxResults: 1000,
				})
				if err != nil {
					fmt.Printf("Warning: Failed to fetch epic details: %v\n", err)
					continue
				}

				for _, e := range epics {
					epicMap[e.Key] = Ticket{
						Key:     e.Key,
						Summary: e.Fields.Summary,
						Link:    e.Self,
						Status:  e.Fields.Status.Name,
					}
				}
			}
		}
	}

	return tickets, epicMap, nil
}

// extractEffortDays extracts the effort in days from the custom field map
func extractEffortDays(unknowns map[string]interface{}, effortFieldID string) (float64, bool) {
	if unknowns == nil {
		return 0, false
	}

	val, ok := unknowns[effortFieldID]
	if !ok || val == nil {
		return 0, false
	}

	// The custom field could be a number (float64 or int) or a string
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		// Try to parse as float
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}
