/*
 * Copyright (c) Cherri Language v2.0
 * Backend Session State Isolation
 */

package main

import (
	"fmt"
	"maps"
	"sync"
)

var (
	compilerSessionMutex     sync.Mutex
	canonicalActionsOnce     sync.Once
	canonicalStandardActions map[string]*actionDefinition
)

func EnsureCanonicalStandardActions() map[string]*actionDefinition {
	canonicalActionsOnce.Do(func() {
		loadStandardActions()
		defineRawAction()
		canonicalStandardActions = maps.Clone(actions)
	})
	return canonicalStandardActions
}

// BackendTransaction represents an isolated emission transaction.
type BackendTransaction struct {
	actionStack []actionReference
}

func BeginBackendTransaction() (*BackendTransaction, func()) {
	compilerSessionMutex.Lock()
	resetCompilerState()

	tx := &BackendTransaction{
		actionStack: make([]actionReference, 0, 8),
	}

	cleanup := func() {
		resetCompilerState()
		compilerSessionMutex.Unlock()
	}

	return tx, cleanup
}

func (tx *BackendTransaction) PushAction(ref actionReference) {
	tx.actionStack = append(tx.actionStack, currentAction)
	currentAction = ref
}

func (tx *BackendTransaction) PopAction() {
	if len(tx.actionStack) > 0 {
		idx := len(tx.actionStack) - 1
		currentAction = tx.actionStack[idx]
		tx.actionStack = tx.actionStack[:idx]
	}
}

func SafeExecute(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("canonical backend panic recovered: %v", r)
		}
	}()
	return fn()
}
