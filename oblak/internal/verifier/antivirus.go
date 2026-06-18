package verifier

import (
    "bytes"
    "fmt"
    "os/exec"
    "strings"
    "time"

    "github.com/milicaradic/oblak/internal/models"
)

func ScanFile(path string) models.AVResult {
    // Pokušaj 1: clamscan CLI
    result, err := runClamscan(path)
    if err == nil {
        return result
    }

    // ClamAV nije dostupan
    fmt.Printf("[WARN] ClamAV nedostupan: %v\n", err)
    return models.AVResult{Clean: true, Detail: "AV not available"}
}

func runClamscan(path string) (models.AVResult, error) {
    cmd := exec.Command("clamscan", "--no-summary", path)

    var out, stderr bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &stderr

    done := make(chan error, 1)
    if err := cmd.Start(); err != nil {
        return models.AVResult{}, err
    }

    go func() { done <- cmd.Wait() }()

    select {
    case err := <-done:
        output := out.String()
        if err == nil {
            return models.AVResult{Clean: true, Detail: "Clean"}, nil
        }
        // Exit code 1 = virus pronađen
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
            for _, line := range strings.Split(output, "\n") {
                if strings.Contains(line, "FOUND") {
                    return models.AVResult{Clean: false, Detail: line}, nil
                }
            }
            return models.AVResult{Clean: false, Detail: "Malware detected"}, nil
        }
        return models.AVResult{Clean: true, Detail: "AV error (permissive)"}, nil

    case <-time.After(30 * time.Second):
        cmd.Process.Kill()
        return models.AVResult{Clean: false, Detail: "AV timeout"}, nil
    }
}