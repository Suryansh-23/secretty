package main

import (
	"fmt"
	"os"
)

func main() {
	state := &appState{}
	rootCmd := newRootCmd(state)
	if err := rootCmd.Execute(); err != nil {
		if exitErr, ok := err.(*exitCodeError); ok {
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
