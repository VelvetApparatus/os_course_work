package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type actorA struct {
	d actorAData
}

type actorAData struct {
	osVersion      string
	keyboardLayout string
}

func (d *actorAData) Report() string {
	return fmt.Sprintf(
		"\n===============\nREPORT\nOS_VERSION: %s\nKEYBOARD_LAYOUT: %s\n===============\n",
		d.osVersion, d.keyboardLayout,
	)
}

func NewWorkerTypeA() Worker {
	return &actorA{}
}

func (aa *actorA) Start(
	ctx context.Context,

) <-chan string {
	reportsCh := make(chan string)

	go func() {
		ticker := time.NewTicker(time.Second * 3)
		defer ticker.Stop()
		defer close(reportsCh)

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				updated, err := aa.update()
				if err != nil {
					continue
				}

				if updated {
					reportsCh <- aa.d.Report()
				}
			}
		}
	}()

	return reportsCh
}

func (aa *actorA) update() (bool, error) {
	layout, err := aa.getKeyboardLayout()
	if err != nil {
		return false, fmt.Errorf("get keyboard layout: %w", err)
	}

	version, err := aa.getOSVersion()
	if err != nil {
		return false, fmt.Errorf("get os version: %w", err)
	}

	updated := aa.d.osVersion != version || aa.d.keyboardLayout != layout

	if updated {
		aa.d = actorAData{
			osVersion:      version,
			keyboardLayout: layout,
		}

	}

	return updated, nil

}

func (aa *actorA) getOSVersion() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// macOS
		out, err := exec.Command("sw_vers", "-productVersion").Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil

	case "linux":
		// Linux (стандартно через /etc/os-release)
		out, err := exec.Command("cat", "/etc/os-release").Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(line[13:], `"`), nil
			}
		}
		return "Unknown Linux version", nil

	default:
		return "", fmt.Errorf("неподдерживаемая ОС: %s", runtime.GOOS)
	}
}

func (aa *actorA) getKeyboardLayout() (string, error) {

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("defaults", "read", os.Getenv("HOME")+"/Library/Preferences/com.apple.HIToolbox.plist", "AppleSelectedInputSources")
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("output: %w", err)
		}
		re := regexp.MustCompile(`"KeyboardLayout Name"\s*=\s*([^;]+);`)

		matches := re.FindSubmatch(out)
		if len(matches) < 2 {
			return "", fmt.Errorf("keyboard layout name not found")
		}

		return string(matches[1]), nil
	case "linux":
		cmd := exec.Command("setxkbmap", "-query")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "layout:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "layout:")), nil
			}
		}
		return "", fmt.Errorf("не удалось найти layout")
	default:
		return "", fmt.Errorf("не поддерживается на данной ОС: %s", runtime.GOOS)
	}
}
