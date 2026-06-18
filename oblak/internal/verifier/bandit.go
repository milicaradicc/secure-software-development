package verifier

import (
    "bytes"
    "encoding/json"
    "os/exec"

    "github.com/milicaradic/oblak/internal/models"
)

type banditOutput struct {
    Results []struct {
        LineNumber    int    `json:"line_number"`
        TestID        string `json:"test_id"`
        IssueText     string `json:"issue_text"`
        IssueSeverity string `json:"issue_severity"`
        IssueConfidence string `json:"issue_confidence"`
    } `json:"results"`
}

func RunBandit(scriptPath string) models.BanditResult {
    cmd := exec.Command("bandit", "-f", "json", "-l", "-i", scriptPath)
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Run() 

    var parsed banditOutput
    if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
        return models.BanditResult{}
    }

    result := models.BanditResult{
        TotalIssues: len(parsed.Results),
    }

    for _, r := range parsed.Results {
        switch r.IssueSeverity {
		case "LOW":
    		result.LowCount++
        case "HIGH":
            result.HighCount++
            result.HighIssues = append(result.HighIssues, models.BanditIssue{
                Line:     r.LineNumber,
                Test:     r.TestID,
                Message:  r.IssueText,
                Severity: r.IssueSeverity,
            })
        case "MEDIUM":
            result.MediumCount++
        }
    }

    return result
}