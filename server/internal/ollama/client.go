package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	defaultBaseUrl = "http://localhost:11434"
	modelListPath  = "/api/tags"
	generatePath   = "/api/generate"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type ModelDetails struct {
	ParameterSize string `json:"parameter_size"`
}

type ModelInfo struct {
	Name    string       `json:"name"`
	Model   string       `json:"model"`
	Details ModelDetails `json:"details"`
}

type ListModelResponse struct {
	Models []ModelInfo `json:"models"`
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseUrl
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) ListModels() ([]ModelInfo, error) {
	resp, err := c.httpClient.Get(c.baseURL + modelListPath)

	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	defer resp.Body.Close()

	var response ListModelResponse

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Models, nil
}

func (c *Client) GenerateStream(model string, messages []Message) (*http.Response, error) {

	var prompt strings.Builder

	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	reqBody := GenerateRequest{
		Model:  model,
		Prompt: prompt.String(),
		Stream: true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+generatePath, "application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// func (c *Client) GenerateStreamWithHistory(model string, messages []Message) (*http.Response, error) {
// 	reqBody := GenerateRequest{
// 		Model:    model,
// 		Messages: messages,
// 		Stream:   true,
// 	}

// 	jsonData, err := json.Marshal(reqBody)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to marshal request: %w", err)
// 	}

// 	resp, err := c.httpClient.Post(c.baseURL+generatePath, "application/json",
// 		bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to send request: %w", err)
// 	}

// 	return resp, nil
// }
