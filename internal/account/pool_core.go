package account

import (
	"sort"
	"sync"
	"time"

	"ds2api/internal/config"
)

type Pool struct {
	store                  *config.Store
	mu                     sync.Mutex
	queue                  []string
	inUse                  map[string]int
	totalInUse             int
	waiters                []chan struct{}
	maxInflightPerAccount  int
	recommendedConcurrency int
	maxQueueSize           int
	globalMaxInflight      int
	banned                 map[string]time.Time // accountID -> unban time
}

func NewPool(store *config.Store) *Pool {
	maxPer := 2
	if store != nil {
		maxPer = store.RuntimeAccountMaxInflight()
	}
	p := &Pool{
		store:                 store,
		inUse:                 map[string]int{},
		maxInflightPerAccount: maxPer,
		banned:                map[string]time.Time{},
	}
	p.Reset()
	go p.bannedCleanupLoop()
	return p
}

func (p *Pool) Reset() {
	accounts := p.store.Accounts()
	sort.SliceStable(accounts, func(i, j int) bool {
		iHas := accounts[i].Token != ""
		jHas := accounts[j].Token != ""
		if iHas == jHas {
			return i < j
		}
		return iHas
	})
	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		id := a.Identifier()
		if id != "" {
			ids = append(ids, id)
		}
	}
	if p.store != nil {
		p.maxInflightPerAccount = p.store.RuntimeAccountMaxInflight()
	} else {
		p.maxInflightPerAccount = maxInflightFromEnv()
	}
	recommended := defaultRecommendedConcurrency(len(ids), p.maxInflightPerAccount)
	queueLimit := maxQueueFromEnv(recommended)
	globalLimit := recommended
	if p.store != nil {
		queueLimit = p.store.RuntimeAccountMaxQueue(recommended)
		globalLimit = p.store.RuntimeGlobalMaxInflight(recommended)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.drainWaitersLocked()
	p.queue = ids
	p.inUse = map[string]int{}
	p.totalInUse = 0
	p.recommendedConcurrency = recommended
	p.maxQueueSize = queueLimit
	p.globalMaxInflight = globalLimit

	// Rotate queue so next account after LastPolledAccountID is at front
	if p.store != nil {
		lastID := p.store.LastPolledAccountID()
		if lastID != "" {
			p.rotateQueueAfter(lastID)
		}
	}

	config.Logger.Info(
		"[init_account_queue] initialized",
		"total", len(ids),
		"max_inflight_per_account", p.maxInflightPerAccount,
		"global_max_inflight", p.globalMaxInflight,
		"recommended_concurrency", p.recommendedConcurrency,
		"max_queue_size", p.maxQueueSize,
		"last_polled", p.store.LastPolledAccountID(),
	)
}

func (p *Pool) Release(accountID string) {
	if accountID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	count := p.inUse[accountID]
	if count <= 0 {
		return
	}
	if count == 1 {
		delete(p.inUse, accountID)
		p.totalInUse--
		p.notifyWaitersLocked()
		// Save last polled account ID for resume-on-restart
		if p.store != nil {
			_ = p.store.SetLastPolledAccountID(accountID)
		}
		return
	}
	p.inUse[accountID] = count - 1
	p.totalInUse--
	p.notifyWaitersLocked()
}

func (p *Pool) Status() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	available := make([]string, 0, len(p.queue))
	inUseAccounts := make([]string, 0, len(p.inUse))
	inUseSlots := p.totalInUse
	for _, id := range p.queue {
		if p.inUse[id] < p.maxInflightPerAccount {
			available = append(available, id)
		}
	}
	for id, count := range p.inUse {
		if count > 0 {
			inUseAccounts = append(inUseAccounts, id)
		}
	}
	sort.Strings(inUseAccounts)
	return map[string]any{
		"available":                len(available),
		"in_use":                   inUseSlots,
		"total":                    len(p.store.Accounts()),
		"available_accounts":       available,
		"in_use_accounts":          inUseAccounts,
		"max_inflight_per_account": p.maxInflightPerAccount,
		"global_max_inflight":      p.globalMaxInflight,
		"recommended_concurrency":  p.recommendedConcurrency,
		"waiting":                  len(p.waiters),
		"max_queue_size":           p.maxQueueSize,
	}
}

// BanAccount 踢出账号指定时长，期间不会分配给任何请求
func (p *Pool) BanAccount(accountID string, duration time.Duration) {
	if accountID == "" || duration <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	banUntil := time.Now().Add(duration)
	p.banned[accountID] = banUntil
	config.Logger.Warn("[pool] account banned", "account", accountID, "duration", duration)
	p.notifyWaitersLocked()
}

// bannedCleanupLoop 后台goroutine，定期清理已过期的banned账号
func (p *Pool) bannedCleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		for id, until := range p.banned {
			if now.After(until) || now.Equal(until) {
				delete(p.banned, id)
				config.Logger.Info("[pool] account unbanned", "account", id)
			}
		}
		p.mu.Unlock()
	}
}

// isBannedLocked 检查账号是否被踢出（调用方需持有锁）
func (p *Pool) isBannedLocked(accountID string) bool {
	if until, ok := p.banned[accountID]; ok {
		if time.Now().Before(until) {
			return true
		}
		// 已过期，清理掉
		delete(p.banned, accountID)
	}
	return false
}

// rotateQueueAfter rotates the queue so that the account after the given account
// is at the front. This is used on startup to resume polling from where we left off.
// For example, if queue is [A, B, C, D] and lastID is "B", the queue becomes [C, D, A, B].
// Caller must hold p.mu.
func (p *Pool) rotateQueueAfter(lastID string) {
	// Find the index of lastID in the queue
	idx := -1
	for i, id := range p.queue {
		if id == lastID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// lastID not found in queue, no rotation needed
		return
	}
	// Rotate so that the next account after lastID is at the front
	// idx is where lastID is, so next account is at (idx + 1) % len
	// We want to rotate left by idx + 1 positions
	rotateBy := idx + 1
	if rotateBy >= len(p.queue) {
		// Already at or past the end, no rotation needed
		return
	}
	// Rotate left by rotateBy positions
	rotated := make([]string, len(p.queue))
	copy(rotated, p.queue[rotateBy:])
	copy(rotated[len(p.queue)-rotateBy:], p.queue[:rotateBy])
	p.queue = rotated
	config.Logger.Info("[pool] queue rotated", "last_polled", lastID, "new_front", p.queue[0])
}
