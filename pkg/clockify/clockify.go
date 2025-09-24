package clockify

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

type ClockifyConfig struct {
	APIKey      string
	BaseURL     string
	UserURL     string
	WorkspaceID string
	UserID      string
}

type TimeEntry struct {
	Duration    time.Duration
	Description string
	ProjectID   string
}

type ReportTimeEntry struct {
	ID           string `json:"id"`
	Description  string `json:"description"`
	TimeInterval struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"timeInterval"`
	IsLocked bool `json:"isLocked"`
}

type Clockify struct {
	Config ClockifyConfig
}

func NewClockify(config ClockifyConfig) *Clockify {
	return &Clockify{
		Config: config,
	}
}

func (c *Clockify) GetWorkspaceID() (string, error) {
	req, err := c.prepareReq(http.MethodGet, c.Config.UserURL)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			return "", err
		}

		formattedBytes, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return "", err
		}

		return string(formattedBytes), nil
	}

	return "", errors.New("failed to get workspace ID")
}

func (c *Clockify) LogTime(te *TimeEntry) error {
	req, err := c.prepareReq(http.MethodPost, c.Config.BaseURL+"/time-entries")
	if err != nil {
		return err
	}

	now := time.Now().Truncate(time.Minute * 30)
	endTime := now.Format(time.RFC3339)
	startTime := now.Add(-te.Duration).Format(time.RFC3339)

	body := map[string]interface{}{
		"billable":    true,
		"end":         endTime,
		"start":       startTime,
		"projectId":   te.ProjectID,
		"description": te.Description,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (c *Clockify) GetReport(from time.Time, to time.Time) ([]ReportTimeEntry, error) {
	log.Default().Println(c.Config.BaseURL + "user/" + c.Config.UserID + "/time-entries")
	req, err := c.prepareReq(http.MethodGet, c.Config.BaseURL+"user/"+c.Config.UserID+"/time-entries")
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"start":     from.Format(time.RFC3339),
		"end":       to.Format(time.RFC3339),
		"page-size": 1000,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var reportEntries []ReportTimeEntry
		if err := json.Unmarshal(bodyBytes, &reportEntries); err != nil {
			return nil, err
		}

		return reportEntries, nil
	}

	return nil, errors.New("failed to get report, response code: " + resp.Status)
}

func (c *Clockify) prepareReq(method string, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Api-Key", c.Config.APIKey)
	return req, nil
}
