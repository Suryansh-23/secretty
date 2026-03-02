package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/ipc"
	"github.com/suryansh-23/secretty/internal/sessioncontrol"
	"github.com/suryansh-23/secretty/internal/types"
)

const defaultPauseDuration = 3 * time.Minute

func newPauseCmd(state *appState) *cobra.Command {
	var (
		pauseFor   string
		commands   int
		showStatus bool
		resume     bool
	)

	cmd := &cobra.Command{
		Use:   "pause",
		Short: "Temporarily pause secret redaction in the active wrapped session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath := os.Getenv("SECRETTY_SOCKET")
			if socketPath == "" {
				return errors.New("pause requires an active wrapped session; run inside `secretty shell`")
			}
			if err := validatePauseFlags(pauseFor, commands, showStatus, resume); err != nil {
				return err
			}
			if state.cfg.Mode == types.ModeStrict {
				fmt.Fprintln(os.Stderr, "warning: pause temporarily disables strict redaction guarantees for this session")
			}

			switch {
			case resume:
				st, err := ipc.PauseResume(socketPath)
				if err != nil {
					return mapPauseIPCError(err)
				}
				fmt.Println("pause resumed")
				printPauseStatus(st)
				return nil
			case showStatus:
				st, err := ipc.PauseStatusQuery(socketPath)
				if err != nil {
					return mapPauseIPCError(err)
				}
				printPauseStatus(st)
				return nil
			case commands > 0:
				st, err := ipc.PauseCommands(socketPath, commands)
				if err != nil {
					return mapPauseIPCError(err)
				}
				fmt.Printf("paused for next %d command lines\n", commands)
				printPauseStatus(st)
				return nil
			default:
				duration := defaultPauseDuration
				if pauseFor != "" {
					parsed, err := time.ParseDuration(pauseFor)
					if err != nil {
						return fmt.Errorf("invalid --for duration: %w", err)
					}
					if parsed <= 0 {
						return errors.New("--for duration must be greater than zero")
					}
					duration = parsed
				}
				st, err := ipc.PauseFor(socketPath, duration)
				if err != nil {
					return mapPauseIPCError(err)
				}
				fmt.Printf("paused for %s\n", duration.Round(time.Second))
				printPauseStatus(st)
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&pauseFor, "for", "", "duration to pause redaction (e.g. 30s, 3m, 1h)")
	cmd.Flags().IntVar(&commands, "commands", 0, "pause redaction for the next N command lines")
	cmd.Flags().BoolVar(&showStatus, "status", false, "show current pause state")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume redaction immediately")
	return cmd
}

func validatePauseFlags(pauseFor string, commands int, showStatus, resume bool) error {
	if commands < 0 {
		return errors.New("--commands must be greater than zero")
	}
	selected := 0
	if pauseFor != "" {
		selected++
	}
	if commands > 0 {
		selected++
	}
	if showStatus {
		selected++
	}
	if resume {
		selected++
	}
	if selected <= 1 {
		return nil
	}
	return errors.New("use only one of: --for, --commands, --status, --resume")
}

func mapPauseIPCError(err error) error {
	if errors.Is(err, ipc.ErrUnsupportedOperation) {
		return errors.New("pause requires a refreshed SecreTTY wrapper; restart your shell or run `secretty shell` again")
	}
	return err
}

func printPauseStatus(st ipc.PauseStatus) {
	if !st.Active {
		fmt.Println("pause inactive")
		return
	}

	switch st.Mode {
	case sessioncontrol.ModeTime:
		remaining := time.Duration(st.RemainingSeconds) * time.Second
		if remaining < 0 {
			remaining = 0
		}
		fmt.Printf("pause active: mode=time remaining=%s\n", remaining.Round(time.Second))
	case sessioncontrol.ModeCommands:
		fmt.Printf("pause active: mode=commands remaining=%d\n", st.RemainingCommands)
	default:
		fmt.Printf("pause active: mode=%s\n", st.Mode)
	}
}
