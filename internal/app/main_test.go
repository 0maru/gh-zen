package app

import (
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain pins lipgloss to a colorless renderer so View output is
// deterministic across hosts and CI runners (no $TERM/$NO_COLOR drift).
func TestMain(m *testing.M) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.Ascii)
	lipgloss.SetDefaultRenderer(r)
	os.Exit(m.Run())
}
