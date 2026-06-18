package orchestrator

import (
	"context"
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "net"
    "net/http"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/milicaradic/oblak/internal/models"
)

type VM struct {
    ID         string
    SocketPath string
    RootfsPath string
    LogPath    string
    process    *exec.Cmd
}

func NewVM() *VM {
    id := uuid.New().String()[:8]
    return &VM{
        ID:         id,
        SocketPath: fmt.Sprintf("%s/%s.sock", SocketBase, id),
        LogPath:    fmt.Sprintf("/tmp/fc_%s.log", id),
    }
}

func (vm *VM) apiCall(method, path string, body any) error {
    var bodyBytes []byte
    var err error

    if body != nil {
        bodyBytes, err = json.Marshal(body)
        if err != nil {
            return err
        }
    }

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", vm.SocketPath)
		},
	}
    client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

    req, err := http.NewRequest(method, "http://localhost"+path, bytes.NewReader(bodyBytes))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("API poziv %s %s neuspeo: %w", method, path, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
        return fmt.Errorf("Firecracker API: status %d za %s %s", resp.StatusCode, method, path)
    }
    return nil
}

func (vm *VM) startProcess() error {
    os.MkdirAll(SocketBase, 0750)
    logFile, err := os.Create(vm.LogPath)
    if err != nil {
        return err
    }

    vm.process = exec.Command(
        FirecrackerBin,
        "--api-sock", vm.SocketPath,
        "--level", "Warning",
    )
    vm.process.Stdout = logFile
    vm.process.Stderr = logFile

    if err := vm.process.Start(); err != nil {
        return fmt.Errorf("pokretanje Firecracker: %w", err)
    }

    for i := 0; i < 20; i++ {
        if _, err := os.Stat(vm.SocketPath); err == nil {
            time.Sleep(50 * time.Millisecond) 
            return nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return fmt.Errorf("Firecracker socket nije kreiran")
}

func (vm *VM) configure() error {
    if err := vm.apiCall("PUT", "/machine-config", MachineConfig{
        VCPUCount:  1,
        MemSizeMiB: 128,
        SMT:        false,
    }); err != nil {
        return err
    }

    if err := vm.apiCall("PUT", "/boot-source", BootSource{
        KernelImagePath: KernelImage,
        BootArgs:        BootArgs,
    }); err != nil {
        return err
    }

    if err := vm.apiCall("PUT", "/drives/rootfs", DriveConfig{
        DriveID:      "rootfs",
        PathOnHost:   vm.RootfsPath,
        IsRootDevice: true,
        IsReadOnly:   false,
    }); err != nil {
        return err
    }

    return nil
}

func (vm *VM) boot() error {
    return vm.apiCall("PUT", "/actions", Action{ActionType: "InstanceStart"})
}

func (vm *VM) collectOutput(timeout time.Duration) (string, int) {
    deadline := time.Now().Add(timeout)
    exitCode := -1
    var outputLines []string

    for time.Now().Before(deadline) {
        content, err := os.ReadFile(vm.LogPath)
        if err == nil {
            text := string(content)
            if strings.Contains(text, "[INIT] Exit:") {
                scanner := bufio.NewScanner(strings.NewReader(text))
                for scanner.Scan() {
                    line := scanner.Text()
                    if strings.Contains(line, "[INIT] Exit:") {
                        parts := strings.SplitN(line, ":", 2)
                        if len(parts) == 2 {
                            fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &exitCode)
                        }
                    } else if !strings.HasPrefix(line, "[INIT]") {
                        outputLines = append(outputLines, line)
                    }
                }
                return strings.Join(outputLines, "\n"), exitCode
            }
        }
        time.Sleep(200 * time.Millisecond)
    }

    return "", 124 
}

func (vm *VM) cleanup() {
    if vm.process != nil {
        vm.process.Process.Signal(os.Interrupt)
        done := make(chan error, 1)
        go func() { done <- vm.process.Wait() }()
        select {
        case <-done:
        case <-time.After(3 * time.Second):
            vm.process.Process.Kill()
        }
    }
    os.Remove(vm.RootfsPath)
    os.Remove(vm.SocketPath)
    os.Remove(vm.LogPath)
}

func RunInVM(funcDir string) (models.InvokeResponse, error) {
    vm := NewVM()
    start := time.Now()

    rootfsPath, err := PrepareRootfs(vm.ID, funcDir)
    if err != nil {
        return models.InvokeResponse{}, fmt.Errorf("priprema rootfs: %w", err)
    }
    vm.RootfsPath = rootfsPath
    defer vm.cleanup()

    if err := vm.startProcess(); err != nil {
        return models.InvokeResponse{}, err
    }
    if err := vm.configure(); err != nil {
        return models.InvokeResponse{}, err
    }
    if err := vm.boot(); err != nil {
        return models.InvokeResponse{}, err
    }

    output, exitCode := vm.collectOutput(VMTimeoutSec * time.Second)
    duration := time.Since(start).Milliseconds()

    return models.InvokeResponse{
        Output:          output,
        ExitCode:        exitCode,
        ExecutionTimeMs: duration,
        TimedOut:        exitCode == 124,
    }, nil
}