package security

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrIPRateLimited   = errors.New("ip login rate limited")
	ErrPairRateLimited = errors.New("ip and username login rate limited")
	ErrBcryptBusy      = errors.New("bcrypt workers busy")
)

type Config struct {
	IPLimit          int
	IPWindow         time.Duration
	PairLimit        int
	PairWindow       time.Duration
	BcryptConcurrent int
}

type windowEntry struct {
	windowStart time.Time
	count       int
}

// LoginProtector 提供进程内的登录防滥用保护。后续可使用 Redis 实现集群级限流，
// 无需改变处理器的保护流程。
type LoginProtector struct {
	mu     sync.Mutex
	ips    map[string]windowEntry
	pairs  map[string]windowEntry
	slots  chan struct{}
	config Config
	now    func() time.Time
	nextGC time.Time
}

func DefaultConfig() Config {
	return Config{
		IPLimit:          200,
		IPWindow:         time.Minute,
		PairLimit:        5,
		PairWindow:       time.Minute,
		BcryptConcurrent: max(1, runtime.NumCPU()*2),
	}
}

func NewLoginProtector(config Config) *LoginProtector {
	if config.IPLimit <= 0 || config.IPWindow <= 0 || config.PairLimit <= 0 ||
		config.PairWindow <= 0 || config.BcryptConcurrent <= 0 {
		panic("invalid login protector config")
	}
	return &LoginProtector{
		ips:    make(map[string]windowEntry),
		pairs:  make(map[string]windowEntry),
		slots:  make(chan struct{}, config.BcryptConcurrent),
		config: config,
		now:    time.Now,
	}
}

func (p *LoginProtector) Allow(ip, username string) error {
	now := p.now()
	username = normalizeUsername(username)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.nextGC.IsZero() || !now.Before(p.nextGC) {
		p.cleanupLocked(now)
		p.nextGC = now.Add(min(p.config.IPWindow, p.config.PairWindow))
	}

	ipState := incrementWindow(p.ips[ip], now, p.config.IPWindow)
	p.ips[ip] = ipState
	if ipState.count > p.config.IPLimit {
		return ErrIPRateLimited
	}

	pairKey := ip + "\x00" + username
	pairState := incrementWindow(p.pairs[pairKey], now, p.config.PairWindow)
	p.pairs[pairKey] = pairState
	if pairState.count > p.config.PairLimit {
		return ErrPairRateLimited
	}
	return nil
}

// Compare 在有界的 CPU 密集型任务池中执行一次密码比较。
// bcrypt 一旦开始执行，就会持续占用槽位直至结束。
func (p *LoginProtector) Compare(ctx context.Context, compare func() error) error {
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrBcryptBusy
	}

	result := make(chan error, 1)
	go func() {
		defer func() { <-p.slots }()
		result <- compare()
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *LoginProtector) ActiveBcrypt() int { return len(p.slots) }

func (p *LoginProtector) cleanupLocked(now time.Time) {
	for ip, state := range p.ips {
		if now.Sub(state.windowStart) >= p.config.IPWindow {
			delete(p.ips, ip)
		}
	}
	for pair, state := range p.pairs {
		if now.Sub(state.windowStart) >= p.config.PairWindow {
			delete(p.pairs, pair)
		}
	}
}

func incrementWindow(state windowEntry, now time.Time, window time.Duration) windowEntry {
	if state.windowStart.IsZero() || now.Sub(state.windowStart) >= window {
		state = windowEntry{windowStart: now}
	}
	state.count++
	return state
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
