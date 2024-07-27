package blossom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func UploadBlob(serverURL string, pubkey string, data []byte) error {
	url := fmt.Sprintf("%s/blossom/upload?pubkey=%s", serverURL, pubkey)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return fmt.Errorf("server error: %s", errorResp.Message)
		}

		return fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	return nil
}

func GetBlob(serverURL string, hash string) ([]byte, error) {
	url := fmt.Sprintf("%s/blossom/%s", serverURL, hash)

	// Send GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return data, nil
}
