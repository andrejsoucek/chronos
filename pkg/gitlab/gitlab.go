package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GitlabConfig struct {
	APIKey  string
	BaseURL string
	UserID  string
}

type Gitlab struct {
	Config *GitlabConfig
}

type LastActivityItem struct {
	Action   string  `json:"action_name"`
	Title    *string `json:"target_title"`
	PushData *struct {
		Ref string `json:"ref"`
	} `json:"push_data"`
	CreatedAt string `json:"created_at"`
}

func NewGitlab(config *GitlabConfig) *Gitlab {
	return &Gitlab{
		Config: config,
	}
}

func (g *Gitlab) GetLastActivity(from time.Time, to time.Time) ([]LastActivityItem, error) {
	req, err := g.prepareReq(
		http.MethodGet,
		g.Config.BaseURL+"users/"+g.Config.UserID+"/events?before="+to.Format(time.RFC3339)+"&after="+from.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}

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

	var response []LastActivityItem
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (g *Gitlab) prepareReq(method string, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("PRIVATE-TOKEN", g.Config.APIKey)
	return req, nil
}
