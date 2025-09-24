package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LinearConfig struct {
	APIKey  string
	BaseURL string
}

type Linear struct {
	Config *LinearConfig
}

type GraphQLRequest struct {
	Query string `json:"query"`
}

type LastActivityItem struct {
	ID         string
	Title      string
	Identifier string
	UpdatedAt  string
}

type LastActivityResponse struct {
	Data struct {
		Issues struct {
			Nodes []LastActivityItem `json:"nodes"`
		} `json:"issues"`
	} `json:"data"`
}

func NewLinear(config *LinearConfig) *Linear {
	return &Linear{
		Config: config,
	}
}

func (l *Linear) GetLastActivity(from time.Time, to time.Time) ([]LastActivityItem, error) {
	graphqlQuery := `
	query myRecentIssueActivity {
		issues(
			first: 50,
			sort: { updatedAt: { order: Descending } },
			filter: {
				and: [
					{
						updatedAt: {
							gte: "` + from.Format(time.RFC3339) + `",
							lte: "` + to.Format(time.RFC3339) + `"
						}
					},
					{
						or: [
							{ creator: { isMe: { eq: true } } },
							{ assignee: { isMe: { eq: true } } },
							{ subscribers: { some: { isMe: { eq: true } } } },
							{ comments: { some: { user: { isMe: { eq: true } } } } }
						]
					}
				]
			}
		) {
			nodes {
				id
				title
				identifier
				updatedAt
				assignee {
					displayName
				}
				creator {
					displayName
				}
			}
		}
	}
	`

	requestBody := GraphQLRequest{
		Query: graphqlQuery,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, l.Config.BaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", l.Config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	var response LastActivityResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response.Data.Issues.Nodes, nil
}
