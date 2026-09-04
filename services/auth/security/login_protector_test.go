package security

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{IPLimit: 3, IPWindow: time.Minute, PairLimit: 2,
		PairWindow: time.Minute, BcryptConcurrent: 1}
}

func TestIPLimit(t *testing.T) {
	p := NewLoginProtector(testConfig())
	for _, username := range []string{"alice", "bob", "carol"} {
		if err := p.Allow("127.0.0.1", username); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Allow("127.0.0.1", "dave"); !errors.Is(err, ErrIPRateLimited) {
		t.Fatalf("expected IP rate limit, got %v", err)
	}
}

func TestPairLimitDoesNotBlockSameUsernameFromAnotherIP(t *testing.T) {
	p := NewLoginProtector(testConfig())
	if err := p.Allow("1", "Alice"); err != nil {
		t.Fatal(err)
	}
	if err := p.Allow("1", " alice "); err != nil {
		t.Fatal(err)
	}
	if err := p.Allow("1", "ALICE"); !errors.Is(err, ErrPairRateLimited) {
		t.Fatalf("expected pair rate limit, got %v", err)
	}
	if err := p.Allow("2", "alice"); err != nil {
		t.Fatalf("another IP should not be blocked: %v", err)
	}
}

func TestPairLimitExpires(t *testing.T) {
	p := NewLoginProtector(testConfig())
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	if err := p.Allow("1", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := p.Allow("1", "alice"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := p.Allow("1", "alice"); err != nil {
		t.Fatalf("pair limit should expire: %v", err)
	}
}

func TestBcryptConcurrencyBound(t *testing.T) {
	p := NewLoginProtector(testConfig())
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- p.Compare(context.Background(), func() error { <-release; return nil }) }()
	deadline := time.Now().Add(time.Second)
	for p.ActiveBcrypt() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := p.Compare(context.Background(), func() error { return nil }); !errors.Is(err, ErrBcryptBusy) {
		t.Fatalf("expected busy, got %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCompareHonorsTimeout(t *testing.T) {
	p := NewLoginProtector(testConfig())
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err := p.Compare(ctx, func() error { time.Sleep(20 * time.Millisecond); return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
}
