package verifier

import (
    "os"
    "strings"
	"github.com/milicaradic/oblak/internal/models"
)

var dangerousPatterns = []struct {
    pattern string
    reason  string
}{
    {"os.fork", "fork bomb mogućnost"},
    {"shutil.rmtree", "brisanje fajlova"},
    {"shutil.rmdir", "brisanje direktorijuma"},
    {"os.remove", "brisanje fajlova"},
    {"os.unlink", "brisanje fajlova"},
    {"while True", "potencijalna beskonačna petlja"},
    {"while 1", "potencijalna beskonačna petlja"},
    {"/etc/passwd", "čitanje sistemskih fajlova"},
    {"/etc/shadow", "čitanje sistemskih fajlova"},
    {"/proc/", "pristup /proc"},
    {"ctypes", "low-level system access"},
    {"import socket", "mrežni pristup"},
    {"socket.socket", "mrežni pristup"},
    {"import subprocess", "pokretanje procesa"},
    {"subprocess.", "pokretanje procesa"},
    {"os.system", "izvršavanje shell komandi"},
    {"eval(", "dinamičko izvršavanje koda"},
    {"exec(", "dinamičko izvršavanje koda"},
    {"urllib", "mrežni pristup"},
    {"requests.", "mrežni pristup"},
}

func checkDangerousPatterns(scriptPath string) (bool, string) {
    content, err := os.ReadFile(scriptPath)
    if err != nil {
        return false, ""
    }
    text := string(content)
    for _, p := range dangerousPatterns {
        if strings.Contains(text, p.pattern) {
            return true, p.reason
        }
    }
    return false, ""
}

func RunPipeline(funcDir, scriptPath string) models.VerificationReport {
    report := models.VerificationReport{}

	// KORAK 0: Custom pattern matching
	blocked, reason := checkDangerousPatterns(scriptPath)
	if blocked {
		report.RejectReason = "Pattern match: " + reason
		return report
	}

    // KORAK 1: Antivirus
    report.Antivirus = ScanFile(scriptPath)
    if !report.Antivirus.Clean {
        report.RejectReason = "Antivirus: " + report.Antivirus.Detail
        return report
    }

    // KORAK 2: Bandit
    report.Bandit = RunBandit(scriptPath)
	if report.Bandit.HighCount > 0 || report.Bandit.MediumCount > 0 || report.Bandit.LowCount > 0 {
		report.RejectReason = "Bandit: pronađeni potencijalno opasni obrasci"
		return report
	}

    report.Passed = true
    return report
}