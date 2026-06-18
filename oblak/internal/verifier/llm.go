package verifier

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"

    "github.com/milicaradic/oblak/internal/models"
)

const systemPrompt = `Ti si ekspert za bezbednost Python koda koji analizira korisničke skripte
pre nego što se izvrše na serveru u izolovanoj Firecracker mikroVM.

UVEK odbij kod koji:
- Pokušava da pobegne iz sandboxa (os.system, subprocess, ctypes)
- Pravi beskonačne petlje ili fork bombe
- Pristupa fajl sistemu van /tmp i /function
- Otvara mrežne konekcije (socket, requests, urllib)
- Koristi eval() ili exec() sa dinamičkim inputom
- Čita /proc, /etc/passwd i slično

Odgovori ISKLJUČIVO u JSON formatu bez ikakvog teksta pre ili posle:
{
  "safe": true/false,
  "confidence": "high"/"medium"/"low",
  "reason": "kratko objašnjenje na srpskom",
  "risks": ["lista konkretnih rizika"],
  "suspicious_lines": [broj_linije_1, broj_linije_2]
}`

type anthropicRequest struct {
    Model     string             `json:"model"`
    MaxTokens int                `json:"max_tokens"`
    System    string             `json:"system"`
    Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type anthropicResponse struct {
    Content []struct {
        Text string `json:"text"`
    } `json:"content"`
}

func LLMReview(scriptPath string) models.LLMResult {
    content, err := os.ReadFile(scriptPath)
    if err != nil {
        return models.LLMResult{Safe: false, Reason: "Ne mogu da pročitam fajl", Confidence: "high"}
    }

    // Dodaj brojeve linija
    lines := strings.Split(string(content), "\n")
    numbered := make([]string, len(lines))
    for i, line := range lines {
        numbered[i] = fmt.Sprintf("%3d | %s", i+1, line)
    }
    numberedCode := strings.Join(numbered, "\n")

    // Napravi request
    reqBody := anthropicRequest{
        Model:     "claude-sonnet-4-6",
        MaxTokens: 1024,
        System:    systemPrompt,
        Messages: []anthropicMessage{
            {
                Role:    "user",
                Content: fmt.Sprintf("Analiziraj sledeći Python kod:\n\n```\n%s\n```", numberedCode),
            },
        },
    }

    reqBytes, _ := json.Marshal(reqBody)

    req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBytes))
    if err != nil {
        return models.LLMResult{Safe: false, Reason: "HTTP greška", Confidence: "low"}
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))
    req.Header.Set("anthropic-version", "2023-06-01")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return models.LLMResult{Safe: false, Reason: fmt.Sprintf("API greška: %v", err), Confidence: "low"}
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var anthropicResp anthropicResponse
    if err := json.Unmarshal(body, &anthropicResp); err != nil || len(anthropicResp.Content) == 0 {
        return models.LLMResult{Safe: false, Reason: "Nevalidan API odgovor", Confidence: "low"}
    }

    var result models.LLMResult
    if err := json.Unmarshal([]byte(anthropicResp.Content[0].Text), &result); err != nil {
        return models.LLMResult{Safe: false, Reason: "LLM vratio nevalidan JSON", Confidence: "low"}
    }

    return result
}