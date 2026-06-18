package audit

import (
    "encoding/json"
    "log"
    "os"
    "time"
)

type Event struct {
    Timestamp string         `json:"timestamp"`
    Event     string         `json:"event"`
    User      string         `json:"user"`
    Details   map[string]any `json:"details"`
}

var auditLogger *log.Logger

func Init() {
    os.MkdirAll("./logs", 0750)
    f, err := os.OpenFile("./logs/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        log.Fatalf("Ne mogu da otvorim audit log: %v", err)
    }
    auditLogger = log.New(f, "", 0)
}

func Log(event, user string, details map[string]any) {
    entry := Event{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Event:     event,
        User:      user,
        Details:   details,
    }
    data, _ := json.Marshal(entry)
    auditLogger.Println(string(data))
}

const (
    EventAuthLoginSuccess  = "AUTH_LOGIN_SUCCESS"
    EventAuthLoginFail     = "AUTH_LOGIN_FAILURE"
    EventAuthRegister      = "AUTH_REGISTER"
    EventFunctionDeploy    = "FUNCTION_DEPLOY"
    EventFunctionVerified  = "FUNCTION_VERIFIED"
    EventFunctionRejected  = "FUNCTION_REJECTED"
    EventFunctionInvoke    = "FUNCTION_INVOKE"
    EventFunctionDelete    = "FUNCTION_DELETE"
    EventSecurityAVBlock   = "SECURITY_AV_BLOCK"
    EventSecurityBandit    = "SECURITY_BANDIT_BLOCK"
    EventSecurityLLMBlock  = "SECURITY_LLM_BLOCK"
)