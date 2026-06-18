package orchestrator

type MachineConfig struct {
    VCPUCount  int  `json:"vcpu_count"`
    MemSizeMiB int  `json:"mem_size_mib"`
    SMT        bool `json:"smt"`
}

type DriveConfig struct {
    DriveID      string `json:"drive_id"`
    PathOnHost   string `json:"path_on_host"`
    IsRootDevice bool   `json:"is_root_device"`
    IsReadOnly   bool   `json:"is_read_only"`
}

type BootSource struct {
    KernelImagePath string `json:"kernel_image_path"`
    BootArgs        string `json:"boot_args"`
}

type Action struct {
    ActionType string `json:"action_type"`
}

const (
    FirecrackerBin = "/usr/local/bin/firecracker"
    KernelImage    = "/opt/firecracker/vmlinux"
    BaseRootfs     = "/opt/firecracker/rootfs.ext4"
    SocketBase     = "/tmp/firecracker"
    VMTimeoutSec   = 15
    BootArgs       = "console=ttyS0 reboot=k panic=1 pci=off nomodules init=/sbin/init_oblak.sh"
)