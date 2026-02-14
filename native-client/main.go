package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martavoi/krts/native-client/internal"
	"github.com/martavoi/krts/native-client/ui"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:4433", "Kratos public URL")
	verbose := flag.Bool("verbose", false, "Verbose request/response logging")
	flag.Parse()

	cfg := Config{
		KratosURL: strings.TrimSuffix(*url, "/"),
		Verbose:  *verbose,
	}

	// Subcommand: signup, signin, whoami (optional)
	var initialFlow ui.FlowType
	args := flag.Args()
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "signup":
			initialFlow = ui.FlowSignup
		case "signin":
			initialFlow = ui.FlowSignin
		case "whoami":
			initialFlow = ui.FlowWhoami
		}
	}

	k := internal.NewKratosClient(cfg.KratosURL, cfg.Verbose)
	m := ui.NewAppModel(k, initialFlow)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
