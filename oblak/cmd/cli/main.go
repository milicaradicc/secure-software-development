package main

import (
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strings"
    "text/tabwriter"
    "time"

    "github.com/spf13/cobra"
)

type Config struct {
    Token    string `json:"token"`
    Username string `json:"username"`
    Server   string `json:"server"`
}

func configPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".oblak", "config.json")
}

func saveConfig(cfg Config) error {
    os.MkdirAll(filepath.Dir(configPath()), 0700)
    data, _ := json.Marshal(cfg)
    return os.WriteFile(configPath(), data, 0600) 
}

func loadConfig() (Config, error) {
    var cfg Config
    data, err := os.ReadFile(configPath())
    if err != nil {
        return cfg, fmt.Errorf("niste ulogovani — pokrenite: oblak login")
    }
    json.Unmarshal(data, &cfg)
    return cfg, nil
}

func serverURL() string {
    if server := os.Getenv("OBLAK_SERVER"); server != "" {
        return server
    }
    return "https://localhost:8000"
}

func boolEnv(name string) bool {
    value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
    return value == "1" || value == "true" || value == "yes"
}

func httpClient(timeout time.Duration) (*http.Client, error) {
    tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

    if boolEnv("OBLAK_INSECURE_SKIP_VERIFY") {
        tlsConfig.InsecureSkipVerify = true
    }

    if caPath := os.Getenv("OBLAK_CA_CERT"); caPath != "" {
        caCert, err := os.ReadFile(caPath)
        if err != nil {
            return nil, fmt.Errorf("ne mogu da procitam CA sertifikat %s: %w", caPath, err)
        }

        pool, err := x509.SystemCertPool()
        if err != nil {
            pool = x509.NewCertPool()
        }
        if !pool.AppendCertsFromPEM(caCert) {
            return nil, fmt.Errorf("CA sertifikat %s nije validan PEM sertifikat", caPath)
        }
        tlsConfig.RootCAs = pool
    }

    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }, nil
}

func ensureHTTPS(endpoint string) error {
    parsed, err := url.Parse(endpoint)
    if err != nil {
        return err
    }
    if parsed.Scheme != "https" {
        return fmt.Errorf("nesiguran URL %q: Oblak komunikacija mora ici preko HTTPS-a", endpoint)
    }
    return nil
}

func doJSON(method, endpoint, token string, payload any) (map[string]any, int, error) {
    if err := ensureHTTPS(endpoint); err != nil {
        return nil, 0, err
    }

    var body io.Reader
    if payload != nil {
        data, _ := json.Marshal(payload)
        body = bytes.NewReader(data)
    }

    req, err := http.NewRequest(method, endpoint, body)
    if err != nil {
        return nil, 0, err
    }
    req.Header.Set("Content-Type", "application/json")
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }

    client, err := httpClient(30 * time.Second)
    if err != nil {
        return nil, 0, err
    }
    resp, err := client.Do(req)
    if err != nil {
        return nil, 0, fmt.Errorf("konekcija neuspela: %w", err)
    }
    defer resp.Body.Close()

    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)
    return result, resp.StatusCode, nil
}

var rootCmd = &cobra.Command{
    Use:   "oblak",
    Short: "Oblak CDK — serverless Python platforma",
}

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Autentikacija ka Oblak serveru",
    RunE: func(cmd *cobra.Command, args []string) error {
        username, _ := cmd.Flags().GetString("username")
        password, _ := cmd.Flags().GetString("password")
        server, _ := cmd.Flags().GetString("server")

        if username == "" {
            fmt.Print("Username: ")
            fmt.Scan(&username)
        }
        if password == "" {
            fmt.Print("Password: ")
            fmt.Scan(&password)
        }
        if server == "" {
            server = serverURL()
        }

        result, status, err := doJSON("POST", server+"/auth/login", "", map[string]string{
            "username": username,
            "password": password,
        })
        if err != nil {
            return err
        }
        if status != 200 {
            return fmt.Errorf("login neuspeo: %v", result["detail"])
        }

        token := result["access_token"].(string)
        saveConfig(Config{Token: token, Username: username, Server: server})
        fmt.Printf("✓ Ulogovani kao %s na %s\n", username, server)
        return nil
    },
}

var registerCmd = &cobra.Command{
    Use:   "register",
    Short: "Registracija novog naloga",
    RunE: func(cmd *cobra.Command, args []string) error {
        username, _ := cmd.Flags().GetString("username")
        password, _ := cmd.Flags().GetString("password")

        if username == "" {
            fmt.Print("Username: ")
            fmt.Scan(&username)
        }
        if password == "" {
            fmt.Print("Password: ")
            fmt.Scan(&password)
        }

        result, status, err := doJSON("POST", serverURL()+"/auth/register", "", map[string]string{
            "username": username,
            "password": password,
        })
        if err != nil {
            return err
        }
        if status != 200 {
            return fmt.Errorf("registracija neuspela: %v", result["detail"])
        }

        fmt.Printf("✓ Nalog kreiran: %s\n", username)
        return nil
    },
}

var deployCmd = &cobra.Command{
    Use:   "deploy <script.py>",
    Short: "Deploy Python skripte na Oblak",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        scriptPath := args[0]
        name, _ := cmd.Flags().GetString("name")
        reqPath, _ := cmd.Flags().GetString("requirements")

        if name == "" {
            base := filepath.Base(scriptPath)
            name = base[:len(base)-len(filepath.Ext(base))]
        }

        fmt.Printf("→ Deploying: %s\n", name)

        var buf bytes.Buffer
        w := multipart.NewWriter(&buf)

        w.WriteField("name", name)

        scriptFile, err := os.Open(scriptPath)
        if err != nil {
            return fmt.Errorf("ne mogu da otvorim %s: %w", scriptPath, err)
        }
        defer scriptFile.Close()

        part, _ := w.CreateFormFile("script", filepath.Base(scriptPath))
        io.Copy(part, scriptFile)

        if reqPath != "" {
            reqFile, err := os.Open(reqPath)
            if err != nil {
                return fmt.Errorf("ne mogu da otvorim %s: %w", reqPath, err)
            }
            defer reqFile.Close()
            reqPart, _ := w.CreateFormFile("requirements", "requirements.txt")
            io.Copy(reqPart, reqFile)
        }
        w.Close()

        deployURL := cfg.Server + "/functions/deploy"
        if err := ensureHTTPS(deployURL); err != nil {
            return err
        }

        req, _ := http.NewRequest("POST", deployURL, &buf)
        req.Header.Set("Content-Type", w.FormDataContentType())
        req.Header.Set("Authorization", "Bearer "+cfg.Token)

        client, err := httpClient(60 * time.Second)
        if err != nil {
            return err
        }
        resp, err := client.Do(req)
        if err != nil {
            return fmt.Errorf("konekcija neuspela: %w", err)
        }
        defer resp.Body.Close()

        var result map[string]any
        json.NewDecoder(resp.Body).Decode(&result)

        if resp.StatusCode != 200 {
            return fmt.Errorf("deploy neuspeo: %v", result["detail"])
        }

        fmt.Println("\n── Verifikacija ─────────────────────────")
        if vr, ok := result["verification_report"].(map[string]any); ok {
            if av, ok := vr["antivirus"].(map[string]any); ok {
                icon := "✓"
                if av["clean"] != true {
                    icon = "✗"
                }
                fmt.Printf("  %s Antivirus: %v\n", icon, av["detail"])
            }
            if b, ok := vr["bandit"].(map[string]any); ok {
                icon := "✓"
                if h, _ := b["high_count"].(float64); h > 0 {
                    icon = "✗"
                }
                fmt.Printf("  %s Bandit: %.0f issues, %.0f HIGH\n", icon, b["total_issues"], b["high_count"])
            }
            if l, ok := vr["llm"].(map[string]any); ok {
                icon := "✓"
                if l["safe"] != true {
                    icon = "✗"
                }
                fmt.Printf("  %s LLM: %v\n", icon, l["reason"])
            }
        }
        fmt.Println("─────────────────────────────────────────")

        if result["verified"] == true {
            invokeURL := cfg.Server + result["invoke_url"].(string)
            fmt.Printf("\n✓ Deploy uspešan!\n")
            fmt.Printf("  Function ID: %v\n", result["function_id"])
            fmt.Printf("  Invoke URL:  %s\n\n", invokeURL)
            fmt.Printf("  Primer:\n")
            fmt.Printf("  $ oblak invoke %s\n", invokeURL)
        } else {
            vr := result["verification_report"].(map[string]any)
            fmt.Printf("\n✗ Verifikacija nije prošla: %v\n", vr["reject_reason"])
            os.Exit(1)
        }

        return nil
    },
}

var invokeCmd = &cobra.Command{
    Use:   "invoke <url>",
    Short: "Pokreni deployjovanu funkciju",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        jsonOutput, _ := cmd.Flags().GetBool("json")
        invokeURL := args[0]
        if err := ensureHTTPS(invokeURL); err != nil {
            return err
        }

        start := time.Now()
        client, err := httpClient(30 * time.Second)
        if err != nil {
            return err
        }
        resp, err := client.Post(invokeURL, "application/json", nil)
        if err != nil {
            return fmt.Errorf("konekcija neuspela: %w", err)
        }
        defer resp.Body.Close()
        duration := time.Since(start).Milliseconds()

        var result map[string]any
        json.NewDecoder(resp.Body).Decode(&result)

        if resp.StatusCode != 200 {
            return fmt.Errorf("invoke neuspeo: %v", result["detail"])
        }

        if jsonOutput {
            data, _ := json.MarshalIndent(result, "", "  ")
            fmt.Println(string(data))
            return nil
        }

        fmt.Println("── Output ───────────────────────────────")
        if output, ok := result["output"].(string); ok && output != "" {
            fmt.Println(output)
        } else {
            fmt.Println("(prazan output)")
        }
        fmt.Println("─────────────────────────────────────────")
        fmt.Printf("Exit: %.0f | Vreme: %dms\n", result["exit_code"], duration)

        if result["timed_out"] == true {
            fmt.Println("⚠ Izvršavanje prekinuto zbog timeout-a")
        }

        return nil
    },
}

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "Prikaz svih deployjovanih funkcija",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        result, status, err := doJSON("GET", cfg.Server+"/functions/", cfg.Token, nil)
        if err != nil {
            return err
        }
        if status != 200 {
            return fmt.Errorf("greška: %v", result["detail"])
        }

        listURL := cfg.Server + "/functions/"
        if err := ensureHTTPS(listURL); err != nil {
            return err
        }

        resp, _ := http.NewRequest("GET", listURL, nil)
        resp.Header.Set("Authorization", "Bearer "+cfg.Token)
        client, err := httpClient(30 * time.Second)
        if err != nil {
            return err
        }
        r, err := client.Do(resp)
        if err != nil {
            return fmt.Errorf("konekcija neuspela: %w", err)
        }
        defer r.Body.Close()

        var functions []map[string]any
        json.NewDecoder(r.Body).Decode(&functions)

        if len(functions) == 0 {
            fmt.Println("Nemaš deployjovanih funkcija.")
            return nil
        }

        tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "ID\tNaziv\tVerified\tKreiran\tURL")
        fmt.Fprintln(tw, "──\t─────\t────────\t───────\t───")
        for _, f := range functions {
            id := f["id"].(string)[:8]
            verified := "✗"
            if f["verified"] == true {
                verified = "✓"
            }
            created := f["created_at"].(string)[:19]
            url := cfg.Server + f["invoke_url"].(string)
            fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", id, f["name"], verified, created, url)
        }
        tw.Flush()
        return nil
    },
}

var deleteCmd = &cobra.Command{
    Use:   "delete <function-id>",
    Short: "Brisanje funkcije",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := loadConfig()
        if err != nil {
            return err
        }

        fmt.Printf("Sigurno želiš da obrišeš %s? [y/N]: ", args[0])
        var confirm string
        fmt.Scan(&confirm)
        if confirm != "y" && confirm != "Y" {
            fmt.Println("Otkazano.")
            return nil
        }

        result, status, err := doJSON("DELETE", cfg.Server+"/functions/"+args[0], cfg.Token, nil)
        if err != nil {
            return err
        }
        if status != 200 {
            return fmt.Errorf("greška: %v", result["detail"])
        }

        fmt.Printf("✓ Funkcija %s obrisana\n", args[0])
        return nil
    },
}

func init() {
    loginCmd.Flags().StringP("username", "u", "", "Username")
    loginCmd.Flags().StringP("password", "p", "", "Password")
    loginCmd.Flags().String("server", "", "Server URL")

    registerCmd.Flags().StringP("username", "u", "", "Username")
    registerCmd.Flags().StringP("password", "p", "", "Password")

    deployCmd.Flags().StringP("name", "n", "", "Naziv funkcije")
    deployCmd.Flags().StringP("requirements", "r", "", "requirements.txt")

    invokeCmd.Flags().Bool("json", false, "JSON output")

    rootCmd.AddCommand(loginCmd, registerCmd, deployCmd, invokeCmd, listCmd, deleteCmd)
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
