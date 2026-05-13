package account

func (p *Pool) canQueueLocked(target string, exclude map[string]bool) bool {
	if target != "" {
		if exclude[target] {
			return false
		}
		if _, ok := p.store.FindAccount(target); !ok {
			return false
		}
	}
	if p.maxQueueSize <= 0 {
		return false
	}
	return len(p.waiters) < p.maxQueueSize
}

func (p *Pool) notifyWaitersLocked() {
	if len(p.waiters) == 0 {
		return
	}
	capacity := p.availableCapacityLocked()
	if capacity <= 0 {
		return
	}
	if capacity > len(p.waiters) {
		capacity = len(p.waiters)
	}
	toNotify := append([]chan struct{}(nil), p.waiters[:capacity]...)
	copy(p.waiters, p.waiters[capacity:])
	for i := len(p.waiters) - capacity; i < len(p.waiters); i++ {
		p.waiters[i] = nil
	}
	p.waiters = p.waiters[:len(p.waiters)-capacity]
	for _, waiter := range toNotify {
		close(waiter)
	}
}

func (p *Pool) availableCapacityLocked() int {
	if len(p.queue) == 0 {
		return 0
	}
	globalCapacity := len(p.queue) * p.maxInflightPerAccount
	if p.globalMaxInflight > 0 && p.globalMaxInflight < globalCapacity {
		globalCapacity = p.globalMaxInflight
	}
	capacity := globalCapacity - p.totalInUse
	if capacity < 0 {
		return 0
	}
	availableAccounts := 0
	for _, id := range p.queue {
		if p.inUse[id] < p.maxInflightPerAccount && !p.isBannedLocked(id) {
			availableAccounts++
		}
	}
	if capacity > availableAccounts {
		return availableAccounts
	}
	return capacity
}

func (p *Pool) removeWaiterLocked(waiter chan struct{}) bool {
	for i, w := range p.waiters {
		if w != waiter {
			continue
		}
		p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
		return true
	}
	return false
}

func (p *Pool) drainWaitersLocked() {
	for _, waiter := range p.waiters {
		close(waiter)
	}
	p.waiters = nil
}
