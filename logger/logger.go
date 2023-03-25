// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package logger

import (
	"go.uber.org/zap"
)

var myLogger *zap.SugaredLogger

// Set sets a global logger
func Set(logger *zap.SugaredLogger) {
	myLogger = logger
}

func I() *zap.SugaredLogger {
	return myLogger
}

func init() {
	Set(zap.NewNop().Sugar())
}
