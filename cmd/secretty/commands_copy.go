package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/clipboard"
	"github.com/suryansh-23/secretty/internal/ipc"
	"github.com/suryansh-23/secretty/internal/types"
)

type copyEntry struct {
	ID        int
	Label     string
	RuleName  string
	Type      types.SecretType
	CreatedAt time.Time
}

type copyResult struct {
	ID       int
	Label    string
	RuleName string
	Type     types.SecretType
}

func newCopyCmd(state *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy redacted secrets without rendering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return errors.New("use `secretty copy last` or `secretty copy pick`")
		},
	}
	cmd.AddCommand(newCopyLastCmd(state))
	cmd.AddCommand(newCopyPickCmd(state))
	return cmd
}

func newCopyLastCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "last",
		Short: "Copy the last redacted secret without rendering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopyLast(state)
		},
	}
}

func newCopyPickCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "pick",
		Short: "Select a cached secret to copy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureCopyAllowed(state); err != nil {
				return err
			}
			entries, err := listCachedSecrets(state)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return errors.New("no secrets cached")
			}
			var selectedID int
			options := make([]huh.Option[int], 0, len(entries))
			for _, entry := range entries {
				label := labelForCopy(entry.Label, entry.RuleName, entry.Type)
				meta := label
				if entry.Type != "" {
					meta = fmt.Sprintf("%s (%s)", label, entry.Type)
				}
				if !entry.CreatedAt.IsZero() {
					age := time.Since(entry.CreatedAt).Truncate(time.Second)
					meta = fmt.Sprintf("%s · %s ago", meta, age)
				}
				options = append(options, huh.NewOption(meta, entry.ID))
			}
			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[int]().Title("Select secret to copy").Options(options...).Value(&selectedID),
			))
			if err := form.Run(); err != nil {
				return err
			}
			if selectedID == 0 {
				return errors.New("no secret selected")
			}
			if state.cfg.Overrides.CopyWithoutRender.RequireConfirm {
				label := labelForCopyLabel(entries, selectedID)
				confirm := false
				form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(fmt.Sprintf("Copy %s to clipboard?", label)).Value(&confirm)))
				if err := form.Run(); err != nil {
					return err
				}
				if !confirm {
					return errors.New("copy cancelled")
				}
			}
			resp, err := copyByID(state, selectedID)
			if err != nil {
				return err
			}
			printCopyResult(state, resp)
			return nil
		},
	}
}

func ensureCopyAllowed(state *appState) error {
	if !state.cfg.Overrides.CopyWithoutRender.Enabled {
		return errors.New("copy-without-render is disabled")
	}
	if state.cfg.Mode == types.ModeStrict && state.cfg.Strict.DisableCopyOriginal {
		return errors.New("copy original is disabled in strict mode")
	}
	return nil
}

func runCopyLast(state *appState) error {
	if err := ensureCopyAllowed(state); err != nil {
		return err
	}
	if state.cfg.Overrides.CopyWithoutRender.RequireConfirm {
		confirm := false
		form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Copy last secret to clipboard?").Value(&confirm)))
		if err := form.Run(); err != nil {
			return err
		}
		if !confirm {
			return errors.New("copy cancelled")
		}
	}
	resp, err := copyLast(state)
	if err != nil {
		return err
	}
	printCopyResult(state, resp)
	return nil
}

func copyLast(state *appState) (copyResult, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		resp, err := ipc.CopyLast(socketPath)
		if err != nil {
			return copyResult{}, err
		}
		return copyResult{ID: resp.ID, Label: resp.Label, RuleName: resp.RuleName, Type: types.SecretType(resp.Type)}, nil
	}
	if state.cache == nil {
		return copyResult{}, errors.New("no secret cache available")
	}
	record, ok := state.cache.GetLast()
	if !ok {
		return copyResult{}, errors.New("no secrets cached")
	}
	if err := clipboard.CopyBytes(state.cfg.Overrides.CopyWithoutRender.Backend, record.Original); err != nil {
		return copyResult{}, err
	}
	return copyResult{ID: record.ID, Label: record.Label, RuleName: record.RuleName, Type: record.Type}, nil
}

func copyByID(state *appState, id int) (copyResult, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		resp, err := ipc.CopyByID(socketPath, id)
		if err != nil {
			return copyResult{}, err
		}
		return copyResult{ID: resp.ID, Label: resp.Label, RuleName: resp.RuleName, Type: types.SecretType(resp.Type)}, nil
	}
	if state.cache == nil {
		return copyResult{}, errors.New("no secret cache available")
	}
	record, ok := state.cache.Get(id)
	if !ok {
		return copyResult{}, errors.New("secret not found")
	}
	if err := clipboard.CopyBytes(state.cfg.Overrides.CopyWithoutRender.Backend, record.Original); err != nil {
		return copyResult{}, err
	}
	return copyResult{ID: record.ID, Label: record.Label, RuleName: record.RuleName, Type: record.Type}, nil
}

func listCachedSecrets(state *appState) ([]copyEntry, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		records, err := ipc.ListSecrets(socketPath)
		if err != nil {
			if errors.Is(err, ipc.ErrUnsupportedOperation) {
				return nil, errors.New("copy pick requires a refreshed SecreTTY wrapper; restart your shell or run `secretty shell` again")
			}
			return nil, err
		}
		out := make([]copyEntry, 0, len(records))
		for _, rec := range records {
			out = append(out, copyEntry{
				ID:        rec.ID,
				Label:     rec.Label,
				RuleName:  rec.RuleName,
				Type:      types.SecretType(rec.Type),
				CreatedAt: rec.CreatedAt,
			})
		}
		return out, nil
	}
	if state.cache == nil {
		return nil, errors.New("no secret cache available")
	}
	records := state.cache.List()
	out := make([]copyEntry, 0, len(records))
	for _, rec := range records {
		out = append(out, copyEntry{
			ID:        rec.ID,
			Label:     rec.Label,
			RuleName:  rec.RuleName,
			Type:      rec.Type,
			CreatedAt: rec.CreatedAt,
		})
	}
	return out, nil
}

func labelForCopy(label, rule string, secretType types.SecretType) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	rule = strings.TrimSpace(rule)
	if rule != "" {
		return rule
	}
	if secretType != "" {
		return string(secretType)
	}
	return "secret"
}

func labelForCopyLabel(entries []copyEntry, id int) string {
	for _, entry := range entries {
		if entry.ID == id {
			return labelForCopy(entry.Label, entry.RuleName, entry.Type)
		}
	}
	return "secret"
}

func printCopyResult(state *appState, resp copyResult) {
	label := labelForCopy(resp.Label, resp.RuleName, resp.Type)
	if state.cfg.Redaction.IncludeEventID && resp.ID > 0 {
		fmt.Printf("Copied %s (%d) to clipboard\n", label, resp.ID)
		return
	}
	fmt.Printf("Copied %s to clipboard\n", label)
}
