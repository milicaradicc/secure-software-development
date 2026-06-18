package orchestrator

import (
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
)

func PrepareRootfs(vmID, funcDir string) (string, error) {
    rootfsPath := fmt.Sprintf("/tmp/rootfs_%s.ext4", vmID)

    if err := copyFile(BaseRootfs, rootfsPath); err != nil {
        return "", fmt.Errorf("kopiranje rootfs: %w", err)
    }

    mountPoint := fmt.Sprintf("/tmp/mount_%s", vmID)
    if err := os.MkdirAll(mountPoint, 0750); err != nil {
        return "", err
    }

    mountCmd := exec.Command("sudo", "mount", "-o", "loop", rootfsPath, mountPoint)
    if out, err := mountCmd.CombinedOutput(); err != nil {
        return "", fmt.Errorf("montiranje neuspelo: %s: %w", string(out), err)
    }
    defer func() {
        exec.Command("sudo", "umount", mountPoint).Run()
        os.Remove(mountPoint)
    }()

    dest := filepath.Join(mountPoint, "function")
    if err := os.MkdirAll(dest, 0750); err != nil {
        return "", err
    }

    for _, fname := range []string{"script.py", "requirements.txt"} {
        src := filepath.Join(funcDir, fname)
        if _, err := os.Stat(src); os.IsNotExist(err) {
            continue
        }
        dst := filepath.Join(dest, fname)
        if err := copyFile(src, dst); err != nil {
            return "", fmt.Errorf("kopiranje %s: %w", fname, err)
        }

		os.Chown(dst, 1000, 1000)
    }

    return rootfsPath, nil
}

func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()

    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, in)
    return err
}

func CleanupRootfs(vmID string) {
    os.Remove(fmt.Sprintf("/tmp/rootfs_%s.ext4", vmID))
}