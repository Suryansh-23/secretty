package main

import (
	"fmt"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/debug"
)

type appState struct {
	cfg      config.Config
	cfgFound bool
	cache    *cache.Cache
	logger   *debug.Logger
	cfgPath  string
}

type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.code)
}
