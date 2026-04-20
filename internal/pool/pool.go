package pool

import (
	"errors"
	"sync"
	"time"

	"xfyun2openai/internal/config"
)

var ErrNoAvailableApp = errors.New("no available app")

type Pool struct {
	apps      map[string]*appRuntime
	modelApps map[string][]string
	selectors map[string]*Selector
}

type Lease struct {
	Name string

	app *appRuntime
}

type appRuntime struct {
	name      string
	cfg       config.AppConfig
	cooldown  time.Duration
	mu        sync.Mutex
	inFlight  int
	failCount int
	coolUntil time.Time
}

func New(cfg *config.Config) *Pool {
	p := &Pool{
		apps:      make(map[string]*appRuntime, len(cfg.Apps)),
		modelApps: make(map[string][]string, len(cfg.Models)),
		selectors: make(map[string]*Selector, len(cfg.Models)),
	}

	cooldown := time.Duration(cfg.Routing.CooldownSeconds) * time.Second
	for name, app := range cfg.Apps {
		p.apps[name] = &appRuntime{
			name:     name,
			cfg:      app,
			cooldown: cooldown,
		}
	}
	for modelName, model := range cfg.Models {
		apps := make([]string, len(model.Apps))
		copy(apps, model.Apps)
		p.modelApps[modelName] = apps
		p.selectors[modelName] = NewSelector()
	}

	return p
}

func (p *Pool) Acquire(modelName string, tried map[string]struct{}) (*Lease, error) {
	appNames, ok := p.modelApps[modelName]
	if !ok {
		return nil, ErrNoAvailableApp
	}

	selector := p.selectors[modelName]
	order := selector.Order(len(appNames))
	now := time.Now()
	for _, index := range order {
		appName := appNames[index]
		if _, skipped := tried[appName]; skipped {
			continue
		}
		app := p.apps[appName]
		if app.tryAcquire(now) {
			selector.Advance(index, len(appNames))
			return &Lease{
				Name: app.name,
				app:  app,
			}, nil
		}
	}

	return nil, ErrNoAvailableApp
}

func (l *Lease) Config() config.AppConfig {
	return l.app.cfg
}

func (l *Lease) Finish(result FinishResult) {
	l.app.finish(result)
}

func (p *Pool) Snapshot(appName string) (AppState, bool) {
	app, ok := p.apps[appName]
	if !ok {
		return AppState{}, false
	}
	return app.snapshot(), true
}

func (a *appRuntime) tryAcquire(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if now.Before(a.coolUntil) {
		return false
	}
	if a.inFlight >= a.cfg.MaxConcurrency {
		return false
	}

	a.inFlight++
	return true
}

func (a *appRuntime) finish(result FinishResult) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.inFlight > 0 {
		a.inFlight--
	}

	if result.Success {
		a.failCount = 0
		a.coolUntil = time.Time{}
		return
	}

	if result.Retryable {
		a.failCount++
	}
	if result.Cooldown {
		a.coolUntil = time.Now().Add(a.cooldown)
	}
}

func (a *appRuntime) snapshot() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()

	return AppState{
		InFlight:      a.inFlight,
		FailCount:     a.failCount,
		CooldownUntil: a.coolUntil,
	}
}
