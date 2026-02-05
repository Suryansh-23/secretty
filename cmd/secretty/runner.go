package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/clipboard"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/debug"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/ipc"
	"github.com/suryansh-23/secretty/internal/ptywrap"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
)

func startIPCServer(cfg config.Config, cache *cache.Cache) (string, func(), error) {
	if cache == nil {
		return "", nil, nil
	}
	if !cfg.Overrides.CopyWithoutRender.Enabled {
		return "", nil, nil
	}
	if cfg.Mode == types.ModeStrict && cfg.Strict.DisableCopyOriginal {
		return "", nil, nil
	}
	socketPath, err := ipc.TempSocketPath()
	if err != nil {
		return "", nil, err
	}
	server, err := ipc.StartServer(socketPath, cache, func(payload []byte) error {
		return clipboard.CopyBytes(cfg.Overrides.CopyWithoutRender.Backend, payload)
	})
	if err != nil {
		_ = os.Remove(socketPath)
		return "", nil, err
	}
	cleanup := func() {
		_ = server.Close()
		_ = os.Remove(socketPath)
	}
	return socketPath, cleanup, nil
}

func runWithPTY(ctx context.Context, cfg config.Config, cfgPath string, command *exec.Cmd, cache *cache.Cache, logger *debug.Logger, interactive bool) error {
	command.Env = os.Environ()
	if os.Getenv("SECRETTY_HOOK_DEBUG") != "" {
		stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
		stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))
		fmt.Fprintf(os.Stderr, "secretty wrapper: interactive=%t stdin_tty=%t stdout_tty=%t cfg=%s cmd=%s\n", interactive, stdinTTY, stdoutTTY, cfgPath, strings.Join(command.Args, " "))
	}
	if os.Getenv("SECRETTY_WRAPPED") == "" {
		command.Env = append(command.Env, "SECRETTY_WRAPPED=1")
	}
	if cfgPath != "" && os.Getenv("SECRETTY_CONFIG") == "" {
		command.Env = append(command.Env, "SECRETTY_CONFIG="+cfgPath)
	}
	cleanup := func() {}
	if cache != nil {
		socketPath, closeFn, err := startIPCServer(cfg, cache)
		if err != nil {
			fmt.Fprintln(os.Stderr, "secretty: copy cache unavailable:", err)
		} else if socketPath != "" {
			command.Env = append(command.Env, "SECRETTY_SOCKET="+socketPath)
			if closeFn != nil {
				cleanup = closeFn
			}
		}
	}
	defer cleanup()
	if interactive {
		cfg.Redaction.RollingWindowBytes = 0
	}
	detector := detect.NewEngine(cfg)
	stream := redact.NewStream(os.Stdout, cfg, detector, cache, logger)
	exitCode, err := ptywrap.RunCommand(ctx, command, ptywrap.Options{
		RawMode: true,
		Output:  stream,
		Logger:  logger,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return &exitCodeError{code: exitCode}
	}
	return nil
}

func ensureCache(existing *cache.Cache, cfg config.Config) *cache.Cache {
	if !cfg.Overrides.CopyWithoutRender.Enabled {
		return nil
	}
	if cfg.Mode == types.ModeStrict && cfg.Strict.DisableCopyOriginal {
		return nil
	}
	ttl := time.Duration(cfg.Overrides.CopyWithoutRender.TTLSeconds) * time.Second
	if existing == nil {
		return cache.New(64, ttl)
	}
	existing.SetTTL(ttl)
	return existing
}
