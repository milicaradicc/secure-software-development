package main

import (
    "crypto/tls"
    "crypto/sha256"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/joho/godotenv"
    "github.com/milicaradic/oblak/internal/audit"
    "github.com/milicaradic/oblak/internal/auth"
    "github.com/milicaradic/oblak/internal/database"
    "github.com/milicaradic/oblak/internal/models"
    "github.com/milicaradic/oblak/internal/orchestrator"
    "github.com/milicaradic/oblak/internal/ratelimit"
    "github.com/milicaradic/oblak/internal/verifier"
)

const (
    StoragePath   = "./functions"
    MaxUploadSize = 1 * 1024 * 1024 
)

func main() {
    godotenv.Load()
    auth.Init()
    database.Init()
    audit.Init()
    os.MkdirAll(StoragePath, 0750)

    r := gin.Default()
    r.MaxMultipartMemory = MaxUploadSize

    authGroup := r.Group("/auth")
    {
        authGroup.POST("/register",
            ratelimit.LoginLimiter.Middleware(func(c *gin.Context) string { return c.ClientIP() }),
            handleRegister,
        )
        authGroup.POST("/login",
            ratelimit.LoginLimiter.Middleware(func(c *gin.Context) string { return c.ClientIP() }),
            handleLogin,
        )
    }

    funcGroup := r.Group("/functions", auth.Middleware())
    {
        funcGroup.POST("/deploy",
            ratelimit.DeployLimiter.Middleware(func(c *gin.Context) string {
                return auth.GetUsername(c)
            }),
            handleDeploy,
        )
        funcGroup.GET("/", handleList)
        funcGroup.DELETE("/:id", handleDelete)
    }

    r.POST("/invoke/:id",
        ratelimit.InvokeLimiter.Middleware(func(c *gin.Context) string { return c.ClientIP() }),
        handleInvoke,
    )

    port := os.Getenv("PORT")
    if port == "" {
        port = "8000"
    }

    if _, err := strconv.Atoi(port); err != nil {
        log.Fatal("Nevalidan PORT (mora biti broj)")
    }

    certFile := os.Getenv("TLS_CERT_FILE")
    if certFile == "" {
        certFile = "./certs/server.crt"
    }
    keyFile := os.Getenv("TLS_KEY_FILE")
    if keyFile == "" {
        keyFile = "./certs/server.key"
    }

    srv := &http.Server{
        Addr:    ":" + port,
        Handler: r,
        ReadHeaderTimeout: 10 * time.Second,
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }

    log.Print("HTTPS server pokrenut")
    if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
        log.Fatalf("HTTPS server nije pokrenut: %v", err)
    }
}

func handleRegister(c *gin.Context) {
    var req models.RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
        return
    }

    var existing models.User
    if result := database.DB.Where("username = ?", req.Username).First(&existing); result.Error == nil {
        c.JSON(http.StatusBadRequest, gin.H{"detail": "Username već postoji"})
        return
    }

    hash, err := auth.HashPassword(req.Password)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"detail": "Greška pri hashiranju"})
        return
    }

    user := models.User{Username: req.Username, HashedPassword: hash}
    database.DB.Create(&user)
    audit.Log(audit.EventAuthRegister, req.Username, nil)
    c.JSON(http.StatusOK, gin.H{"message": "Registracija uspešna"})
}

func handleLogin(c *gin.Context) {
    var req models.LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
        return
    }

    var user models.User
    if result := database.DB.Where("username = ?", req.Username).First(&user); result.Error != nil {
        audit.Log(audit.EventAuthLoginFail, req.Username, map[string]any{"ip": c.ClientIP()})
        c.JSON(http.StatusUnauthorized, gin.H{"detail": "Pogrešni kredencijali"})
        return
    }

    if !auth.CheckPassword(req.Password, user.HashedPassword) {
        audit.Log(audit.EventAuthLoginFail, req.Username, map[string]any{"ip": c.ClientIP()})
        c.JSON(http.StatusUnauthorized, gin.H{"detail": "Pogrešni kredencijali"})
        return
    }

    token, err := auth.CreateToken(user.Username)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"detail": "Greška pri kreiranju tokena"})
        return
    }

    audit.Log(audit.EventAuthLoginSuccess, req.Username, map[string]any{"ip": c.ClientIP()})
    c.JSON(http.StatusOK, models.TokenResponse{AccessToken: token, TokenType: "bearer"})
}

func handleDeploy(c *gin.Context) {
    username := auth.GetUsername(c)

    name := c.PostForm("name")
    if name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"detail": "name je obavezan"})
        return
    }

    scriptFile, err := c.FormFile("script")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"detail": "script fajl je obavezan"})
        return
    }

    if filepath.Ext(scriptFile.Filename) != ".py" {
        c.JSON(http.StatusBadRequest, gin.H{"detail": "Samo .py fajlovi su dozvoljeni"})
        return
    }

    if scriptFile.Size > MaxUploadSize {
        c.JSON(http.StatusBadRequest, gin.H{"detail": "Fajl prevelik"})
        return
    }

    funcID := uuid.New().String()
    funcDir := filepath.Join(StoragePath, funcID)
    os.MkdirAll(funcDir, 0750)

    scriptPath := filepath.Join(funcDir, "script.py")
    if err := c.SaveUploadedFile(scriptFile, scriptPath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"detail": "Greška pri čuvanju fajla"})
        return
    }

    content, _ := os.ReadFile(scriptPath)
    fileHash := fmt.Sprintf("%x", sha256.Sum256(content))

    if reqFile, err := c.FormFile("requirements"); err == nil {
        c.SaveUploadedFile(reqFile, filepath.Join(funcDir, "requirements.txt"))
    }

    audit.Log(audit.EventFunctionDeploy, username, map[string]any{
        "function_id": funcID,
        "name":        name,
        "size_bytes":  scriptFile.Size,
        "sha256":      fileHash,
    })

    report := verifier.RunPipeline(funcDir, scriptPath)

    if !report.Passed {
        audit.Log(audit.EventFunctionRejected, username, map[string]any{
            "function_id": funcID,
            "reason":      report.RejectReason,
        })
    }

    fn := models.Function{
        ID:       funcID,
        Owner:    username,
        Name:     name,
        Path:     funcDir,
        FileHash: fileHash,
        Verified: report.Passed,
    }
    database.DB.Create(&fn)

    c.JSON(http.StatusOK, models.DeployResponse{
        FunctionID:         funcID,
        InvokeURL:          "/invoke/" + funcID,
        Verified:           report.Passed,
        VerificationReport: report,
    })
}

func handleInvoke(c *gin.Context) {
    funcID := c.Param("id")

    var fn models.Function
    if result := database.DB.Where("id = ?", funcID).First(&fn); result.Error != nil {
        c.JSON(http.StatusNotFound, gin.H{"detail": "Funkcija nije pronađena"})
        return
    }

    if !fn.Verified {
        c.JSON(http.StatusForbidden, gin.H{"detail": "Funkcija nije verifikovana"})
        return
    }

    result, err := orchestrator.RunInVM(fn.Path)
    if err != nil {
        audit.Log(audit.EventFunctionInvoke+"_FAILED", "anonymous", map[string]any{
            "function_id": funcID,
            "error":       err.Error(),
        })
        c.JSON(http.StatusInternalServerError, gin.H{"detail": "VM greška: " + err.Error()})
        return
    }

    audit.Log(audit.EventFunctionInvoke, "anonymous", map[string]any{
        "function_id":  funcID,
        "exit_code":    result.ExitCode,
        "duration_ms":  result.ExecutionTimeMs,
    })

    c.JSON(http.StatusOK, result)
}

func handleList(c *gin.Context) {
    username := auth.GetUsername(c)
    var functions []models.Function
    database.DB.Where("owner = ?", username).Find(&functions)

    result := make([]models.FunctionInfo, len(functions))
    for i, f := range functions {
        result[i] = models.FunctionInfo{
            ID:        f.ID,
            Name:      f.Name,
            Verified:  f.Verified,
            CreatedAt: f.CreatedAt,
            InvokeURL: "/invoke/" + f.ID,
        }
    }
    c.JSON(http.StatusOK, result)
}

func handleDelete(c *gin.Context) {
    username := auth.GetUsername(c)
    funcID := c.Param("id")

    var fn models.Function
    if result := database.DB.Where("id = ? AND owner = ?", funcID, username).First(&fn); result.Error != nil {
        c.JSON(http.StatusNotFound, gin.H{"detail": "Funkcija nije pronađena"})
        return
    }

    os.RemoveAll(fn.Path)
    database.DB.Delete(&fn)
    audit.Log(audit.EventFunctionDelete, username, map[string]any{"function_id": funcID})
    c.JSON(http.StatusOK, gin.H{"message": "Obrisano"})
}
