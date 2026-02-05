package redact_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/redact"
)

func TestInteractiveCacheStoresSecretWithTail(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	cfg.Overrides.CopyWithoutRender.Enabled = true
	cfg.Overrides.CopyWithoutRender.TTLSeconds = 30
	cfg.Rulesets.Web3.Enabled = true

	out := &bytes.Buffer{}
	secretCache := cache.New(64, 30*time.Second)
	stream := redact.NewStream(out, cfg, detect.NewEngine(cfg), secretCache, nil)

	if _, err := stream.Write([]byte("prefix output\n")); err != nil {
		t.Fatalf("write prefix: %v", err)
	}

	secret := "0x" + strings.Repeat("a", 64)
	payload := "PRIVATE_KEY=" + secret + "\n"
	if _, err := stream.Write([]byte(payload)); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	rec, ok := secretCache.GetLast()
	if !ok {
		t.Fatal("expected cached secret")
	}
	if string(rec.Original) != secret {
		t.Fatalf("expected secret %q, got %q", secret, string(rec.Original))
	}
}
