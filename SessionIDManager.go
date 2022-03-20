package main

import (
	"sync"

	"github.com/bits-and-blooms/bitset"
)

//////////////////////////////// SessionIDManager //////////////////////////////

// SessionIDManager Thread security session ID manager
type SessionIDManager struct {
	lock       sync.Mutex
	sessionIDs *bitset.BitSet

	count        uint16 // how many ids are used now
	allocIDx     uint16
	maxSessionId uint16 // session IDMaximum value that can be achieved
}

// NewSessionIDManager Create a session ID manager instance
func NewSessionIDManager(maxSessionId uint16) (manager *SessionIDManager, err error) {
	manager = new(SessionIDManager)

	manager.maxSessionId = maxSessionId

	manager.sessionIDs = bitset.New(uint(manager.maxSessionId + 1))
	manager.count = 0
	manager.sessionIDs.ClearAll()
	return
}

// isFull 判断会话ID是否已满（内部使用，不加锁）
func (manager *SessionIDManager) isFullWithoutLock() bool {
	return (manager.count > manager.maxSessionId)
}

// IsFull 判断会话ID是否已满
func (manager *SessionIDManager) IsFull() bool {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	return manager.isFullWithoutLock()
}

func (manager *SessionIDManager) next() {
	manager.allocIDx++
	if manager.allocIDx > manager.maxSessionId {
		manager.allocIDx = 0
	}
}

// AllocSessionID Assign a session ID for the caller
func (manager *SessionIDManager) AllocSessionID() (sessionID uint16, err error) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	if manager.isFullWithoutLock() {
		sessionID = manager.maxSessionId
		err = ErrSessionIDFull
		return
	}

	// find an empty bit
	for manager.sessionIDs.Test(uint(manager.allocIDx)) {
		manager.next()
	}

	// set to true
	manager.sessionIDs.Set(uint(manager.allocIDx))
	manager.count++

	sessionID = manager.allocIDx
	err = nil
	manager.next()
	return
}

// FreeSessionID 释放调用者持有的会话ID
func (manager *SessionIDManager) FreeSessionID(sessionID uint16) {
	defer manager.lock.Unlock()
	manager.lock.Lock()

	if !manager.sessionIDs.Test(uint(sessionID)) {
		// ID未分配，无需释放
		return
	}

	manager.sessionIDs.Clear(uint(sessionID))
	manager.count--
}
