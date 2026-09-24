package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const BaseURL = "https://api.telegram.org/bot"

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type Message struct {
	MessageID int `json:"message_id"`
}

type APIResponse struct {
	OK          bool    `json:"ok"`
	Result      Message `json:"result"`
	Description string  `json:"description"`
	ErrorCode   int     `json:"error_code"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (c *Client) SendDocument(ctx context.Context, chatID string, filePath string, caption string) (*Message, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("chat_id", chatID)
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+c.token+"/sendDocument", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		if result.ErrorCode == 429 && result.Parameters.RetryAfter > 0 {
			time.Sleep(time.Duration(result.Parameters.RetryAfter) * time.Second)
			return c.SendDocument(ctx, chatID, filePath, caption)
		}
		return nil, fmt.Errorf("telegram error %d: %s", result.ErrorCode, result.Description)
	}

	return &result.Result, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID string, text string, parseMode string) (*Message, error) {
	payload := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               parseMode,
		"disable_web_page_preview": true,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+c.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram error %d: %s", result.ErrorCode, result.Description)
	}

	return &result.Result, nil
}

func (c *Client) DeleteMessages(ctx context.Context, chatID string, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}

	for i := 0; i < len(messageIDs); i += 100 {
		end := i + 100
		if end > len(messageIDs) {
			end = len(messageIDs)
		}

		payload := map[string]interface{}{
			"chat_id":     chatID,
			"message_ids": messageIDs[i:end],
		}

		body, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(ctx, "POST", BaseURL+c.token+"/deleteMessages", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}

	return nil
}
