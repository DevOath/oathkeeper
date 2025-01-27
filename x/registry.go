// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package x

import (
	"github.com/ory/x/logrusx"

	"github.com/ory/herodot"
)

type TestLoggerProvider struct{}

func (lp *TestLoggerProvider) Logger() *logrusx.Logger {
	return logrusx.New("", "")
}

type RegistryLogger interface {
	Logger() *logrusx.Logger
}

type RegistryWriter interface {
	Writer() herodot.Writer
}
