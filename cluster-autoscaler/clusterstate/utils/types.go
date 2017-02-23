/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"sync"
	"time"
)

const (
	// DefaultLogCollectorMaxItems defines default maximum size of LogCollector.
	DefaultLogCollectorMaxItems = 50
	// DefaultLogCollectorItemLifetime is the default time after which LogItem will be removed from LogCollector.
	DefaultLogCollectorItemLifetime = 15 * time.Minute
)

type LogLevel int

const (
	// Debug log level.
	Debug = iota
	// Info log level.
	Info = iota
	// Warning log level.
	Warning = iota
	// Error log level.
	Error = iota
)

// LogItem is a single entry in log managed by LogCollector.
type LogItem struct {
	// Log is the logged message body.
	Log string
	// Level describes log severity.
	Level LogLevel
	// Timestamp when the Log was created.
	Timestamp time.Time
}

// LogCollector keeps recent log events. It is automatically truncated on each access based on predefined set of conditions.
type LogCollector struct {
	sync.Mutex
	maxItems     int
	itemLifetime time.Duration
	store        []LogItem
}
