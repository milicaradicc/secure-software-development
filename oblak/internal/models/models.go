package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

type User struct {
    ID             string    `gorm:"primaryKey" json:"id"`
    Username       string    `gorm:"unique;not null" json:"username"`
    HashedPassword string    `gorm:"not null" json:"-"`
    CreatedAt      time.Time `json:"created_at"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
    if u.ID == "" {
        u.ID = uuid.New().String()
    }
    return nil
}

type Function struct {
    ID         string    `gorm:"primaryKey" json:"id"`
    Owner      string    `gorm:"not null;index" json:"owner"`
    Name       string    `gorm:"not null" json:"name"`
    Path       string    `gorm:"not null" json:"-"`
    FileHash   string    `gorm:"not null" json:"file_hash"`
    Verified   bool      `gorm:"default:false" json:"verified"`
    CreatedAt  time.Time `json:"created_at"`
}

func (f *Function) BeforeCreate(tx *gorm.DB) error {
    if f.ID == "" {
        f.ID = uuid.New().String()
    }
    return nil
}

type RegisterRequest struct {
    Username string `json:"username" binding:"required,min=3,alphanum"`
    Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type TokenResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
}

type DeployResponse struct {
    FunctionID          string              `json:"function_id"`
    InvokeURL           string              `json:"invoke_url"`
    Verified            bool                `json:"verified"`
    VerificationReport  VerificationReport  `json:"verification_report"`
}

type InvokeResponse struct {
    Output          string `json:"output"`
    Stderr          string `json:"stderr"`
    ExitCode        int    `json:"exit_code"`
    ExecutionTimeMs int64  `json:"execution_time_ms"`
    TimedOut        bool   `json:"timed_out"`
}

type FunctionInfo struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Verified  bool      `json:"verified"`
    CreatedAt time.Time `json:"created_at"`
    InvokeURL string    `json:"invoke_url"`
}

type VerificationReport struct {
    Passed       bool         `json:"passed"`
    RejectReason string       `json:"reject_reason,omitempty"`
    Antivirus    AVResult     `json:"antivirus"`
    Bandit       BanditResult `json:"bandit"`
}

type AVResult struct {
    Clean  bool   `json:"clean"`
    Detail string `json:"detail"`
}

type BanditResult struct {
    TotalIssues int           `json:"total_issues"`
    HighCount   int           `json:"high_count"`
    MediumCount int           `json:"medium_count"`
    LowCount    int           `json:"low_count"`
    HighIssues  []BanditIssue `json:"high_issues,omitempty"`
}

type BanditIssue struct {
    Line     int    `json:"line"`
    Test     string `json:"test"`
    Message  string `json:"message"`
    Severity string `json:"severity"`
}

type LLMResult struct {
    Safe             bool     `json:"safe"`
    Confidence       string   `json:"confidence"`
    Reason           string   `json:"reason"`
    Risks            []string `json:"risks,omitempty"`
    SuspiciousLines  []int    `json:"suspicious_lines,omitempty"`
}