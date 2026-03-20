package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// httpClient is the shared HTTP client for all outbound requests (Telegram CDN and
// OpenAI API). Using a client with an explicit timeout avoids the pitfall of hanging
// goroutines when a large file download or a slow API call stalls indefinitely.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// maxFileSize is the maximum document size the bot will process (10 MB).
const maxFileSize = 10 * 1024 * 1024

// maxTextChars is the maximum number of characters extracted from text files before
// truncation. This keeps prompts inside Claude's context window.
const maxTextChars = 100_000

// textExtensions is the set of file extensions treated as plain-text content.
var textExtensions = map[string]bool{
	".md":   true,
	".txt":  true,
	".json": true,
	".yaml": true,
	".yml":  true,
	".csv":  true,
	".xml":  true,
	".html": true,
	".css":  true,
	".js":   true,
	".ts":   true,
	".py":   true,
	".sh":   true,
	".env":  true,
	".log":  true,
	".cfg":  true,
	".ini":  true,
	".toml": true,
}

// isTextFile reports whether filename has a text file extension.
// The comparison is case-insensitive so "README.MD" and "readme.md" both match.
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return textExtensions[ext]
}

// downloadToTemp downloads the Telegram file identified by fileID to a temporary
// file with the given suffix (e.g. ".ogg", ".jpg"). Returns the local path.
// The caller is responsible for calling os.Remove on the returned path.
func downloadToTemp(bot *gotgbot.Bot, fileID string, suffix string) (string, error) {
	file, err := bot.GetFile(fileID, nil)
	if err != nil {
		return "", fmt.Errorf("GetFile: %w", err)
	}
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", bot.Token, file.FilePath)
	return downloadFromURL(url, suffix)
}

// downloadFromURL downloads the content at url to a temporary file with the given suffix.
// This is a testable helper extracted from downloadToTemp so tests can supply a mock URL.
func downloadFromURL(url string, suffix string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp("", "tg_*"+suffix)
	if err != nil {
		return "", fmt.Errorf("CreateTemp: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return tmp.Name(), nil
}

// whisperEndpoint is the OpenAI audio transcription endpoint.
// It is a variable (not a constant) so tests can substitute a mock server URL.
var whisperEndpoint = "https://api.openai.com/v1/audio/transcriptions"

// transcribeVoice uploads the audio file at filePath to the OpenAI Whisper API
// and returns the transcribed text. apiKey must be a valid OpenAI API key.
// Returns an error if the API responds with a non-200 status.
func transcribeVoice(ctx context.Context, apiKey string, filePath string) (string, error) {
	return transcribeVoiceURL(ctx, apiKey, filePath, whisperEndpoint)
}

// transcribeVoiceURL is the testable implementation of transcribeVoice.
// It accepts an explicit endpoint URL so tests can substitute a mock HTTP server.
func transcribeVoiceURL(ctx context.Context, apiKey string, filePath string, endpoint string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open voice file: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("copy file to form: %w", err)
	}
	if err := w.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API %d: %s", resp.StatusCode, bytes.TrimSpace(b))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode whisper response: %w", err)
	}
	return result.Text, nil
}

// extractPDF invokes the pdftotext CLI with the -layout flag to extract text from
// the PDF at filePath, writing the output to stdout (the "-" argument).
//
// On exit code 1 from pdftotext (common with encrypted/partially-corrupted PDFs),
// if stdout contains partial output it is returned as success (partial extraction).
// An empty stdout with a non-zero exit code returns an error.
func extractPDF(ctx context.Context, pdfToTextPath string, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, pdfToTextPath, "-layout", filePath, "-")
	out, err := cmd.Output()
	if err != nil {
		// Partial extraction: if pdftotext wrote something before exiting non-zero, use it.
		if exitErr, ok := err.(*exec.ExitError); ok && len(out) > 0 {
			_ = exitErr
			return string(out), nil
		}
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(out), nil
}
