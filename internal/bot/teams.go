package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Application struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type MessageFrom struct {
	Application Application `json:"application"`
}

type MessageBody struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

type Message struct {
	ID      string      `json:"id"`
	From    MessageFrom `json:"from"`
	Body    MessageBody `json:"body"`
	Replies []Message   `json:"replies"`
}

type graphResponse struct {
	Value []Message `json:"value"`
}

type TeamsClient struct {
	http      *http.Client
	teamID    string
	channelID string
}

func NewTeamsClient(httpClient *http.Client, teamID, channelID string) *TeamsClient {
	return &TeamsClient{
		http:      httpClient,
		teamID:    teamID,
		channelID: channelID,
	}
}

func (c *TeamsClient) fetchMessages() ([]Message, error) {
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/teams/%s/channels/%s/messages?$top=10&$expand=replies",
		c.teamID, c.channelID,
	)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Value, nil
}

func (c *TeamsClient) postReply(messageID, content string) error {
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/teams/%s/channels/%s/messages/%s/replies",
		c.teamID, c.channelID, messageID,
	)
	payload := map[string]any{
		"body": map[string]string{"content": content},
	}
	jsonPayload, _ := json.Marshal(payload)
	resp, err := c.http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}
