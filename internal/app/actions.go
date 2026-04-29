package app

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

type actionRunner interface {
	Open(ctx context.Context, target string) error
	Copy(ctx context.Context, text string) error
}

type systemActionRunner struct{}

type actionResultMsg struct {
	success string
	failure string
	err     error
}

func (systemActionRunner) Open(ctx context.Context, target string) error {
	if !isOpenTargetURL(target) {
		return fmt.Errorf("unsupported URL %q", target)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", "--", target)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", target)
	}
	return cmd.Run()
}

func isOpenTargetURL(target string) bool {
	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return parsed.Host != ""
	default:
		return false
	}
}

func (systemActionRunner) Copy(ctx context.Context, text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "pbcopy")
	case "windows":
		cmd = exec.CommandContext(ctx, "clip")
	default:
		return copyWithLinuxClipboard(ctx, text)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func copyWithLinuxClipboard(ctx context.Context, text string) error {
	if err := runClipboardCommand(ctx, text, "wl-copy"); err == nil {
		return nil
	}
	if err := runClipboardCommand(ctx, text, "xclip", "-selection", "clipboard"); err == nil {
		return nil
	}
	return fmt.Errorf("no supported clipboard command found")
}

func runClipboardCommand(ctx context.Context, text string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
