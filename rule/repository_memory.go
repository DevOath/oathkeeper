// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package rule

import (
	"context"
	"net/url"
	"sync"

	"github.com/pkg/errors"

	"github.com/ory/oathkeeper/driver/configuration"
	"github.com/ory/oathkeeper/driver/health"
	"github.com/ory/oathkeeper/helper"
	rulereadiness "github.com/ory/oathkeeper/rule/readiness"
	"github.com/ory/oathkeeper/x"

	"github.com/ory/x/pagination"
)

var _ Repository = new(RepositoryMemory)

type repositoryMemoryRegistry interface {
	RuleValidator() Validator
	x.RegistryLogger
}

type RepositoryMemory struct {
	sync.RWMutex
	rules            []Rule
	matchingStrategy configuration.MatchingStrategy
	r                repositoryMemoryRegistry

	hem health.EventManager
}

// MatchingStrategy returns current MatchingStrategy.
func (m *RepositoryMemory) MatchingStrategy(_ context.Context) (configuration.MatchingStrategy, error) {
	m.RLock()
	defer m.RUnlock()
	return m.matchingStrategy, nil
}

// SetMatchingStrategy updates MatchingStrategy.
func (m *RepositoryMemory) SetMatchingStrategy(_ context.Context, ms configuration.MatchingStrategy) error {
	m.Lock()
	defer m.Unlock()
	m.matchingStrategy = ms
	return nil
}

func NewRepositoryMemory(r repositoryMemoryRegistry, hem health.EventManager) *RepositoryMemory {
	return &RepositoryMemory{
		r:     r,
		rules: make([]Rule, 0),
		hem:   hem,
	}
}

// WithRules sets rules without validation. For testing only.
func (m *RepositoryMemory) WithRules(rules []Rule) {
	m.Lock()
	m.rules = rules
	m.Unlock()
}

func (m *RepositoryMemory) Count(ctx context.Context) (int, error) {
	m.RLock()
	defer m.RUnlock()

	return len(m.rules), nil
}

func (m *RepositoryMemory) List(ctx context.Context, limit, offset int) ([]Rule, error) {
	m.RLock()
	defer m.RUnlock()

	start, end := pagination.Index(limit, offset, len(m.rules))
	return m.rules[start:end], nil
}

func (m *RepositoryMemory) Get(ctx context.Context, id string) (*Rule, error) {
	m.RLock()
	defer m.RUnlock()

	for _, r := range m.rules {
		if r.ID == id {
			return &r, nil
		}
	}

	return nil, errors.WithStack(helper.ErrResourceNotFound)
}

func (m *RepositoryMemory) Set(ctx context.Context, rules []Rule) error {
	for _, check := range rules {
		if err := m.r.RuleValidator().Validate(&check); err != nil {
			m.r.Logger().WithError(err).WithField("rule_id", check.ID).
				Errorf("A Rule uses a malformed configuration and all URLs matching this rule will not work. You should resolve this issue now.")
		}
	}

	m.Lock()
	m.rules = rules
	m.hem.Dispatch(&rulereadiness.RuleLoadedEvent{})
	m.Unlock()
	return nil
}

func (m *RepositoryMemory) Match(ctx context.Context, method string, u *url.URL, protocol Protocol) (*Rule, error) {
	if u == nil {
		return nil, errors.WithStack(errors.New("nil URL provided"))
	}

	m.Lock()
	defer m.Unlock()

	var rules []Rule
	for k := range m.rules {
		r := &m.rules[k]
		if matched, err := r.IsMatching(m.matchingStrategy, method, u, protocol); err != nil {
			return nil, errors.WithStack(err)
		} else if matched {
			rules = append(rules, *r)
		}
		m.rules[k] = *r
	}

	if len(rules) == 0 {
		return nil, errors.WithStack(helper.ErrMatchesNoRule)
	} else if len(rules) != 1 {
		return nil, errors.WithStack(helper.ErrMatchesMoreThanOneRule)
	}

	return &rules[0], nil
}
