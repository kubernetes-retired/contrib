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

package api

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultLogCollectorMaxItems defines default maximum size of LogCollector.
	DefaultLogCollectorMaxItems = 50
	// DefaultLogCollectorItemLifetime is the default time after which LogItem will be removed from LogCollector.
	DefaultLogCollectorItemLifetime = 15 * time.Minute
)

// GetConditionByType gets condition by type.
func GetConditionByType(conditionType ClusterAutoscalerConditionType,
	conditions []ClusterAutoscalerCondition) *ClusterAutoscalerCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func getConditionsString(autoscalerConditions []ClusterAutoscalerCondition, prefix string) string {
	health := fmt.Sprintf("%v%-12v <unknown>", prefix, ClusterAutoscalerHealth+":")
	var scaleUp, scaleDown string
	var buffer, other bytes.Buffer
	for _, condition := range autoscalerConditions {
		var line bytes.Buffer
		line.WriteString(fmt.Sprintf("%v%-12v %v",
			prefix,
			condition.Type+":",
			condition.Status))
		if condition.Message != "" {
			line.WriteString(" (")
			line.WriteString(condition.Message)
			line.WriteString(")")
		}
		line.WriteString("\n")
		line.WriteString(fmt.Sprintf("%v%13sLastProbeTime:      %v\n",
			prefix,
			"",
			condition.LastProbeTime))
		line.WriteString(fmt.Sprintf("%v%13sLastTransitionTime: %v\n",
			prefix,
			"",
			condition.LastTransitionTime))
		switch condition.Type {
		case ClusterAutoscalerHealth:
			health = line.String()
		case ClusterAutoscalerScaleUp:
			scaleUp = line.String()
		case ClusterAutoscalerScaleDown:
			scaleDown = line.String()
		default:
			other.WriteString(line.String())
		}
	}
	buffer.WriteString(health)
	buffer.WriteString(scaleUp)
	buffer.WriteString(scaleDown)
	buffer.WriteString(other.String())
	return buffer.String()
}

// GetReadableString produces human-redable description of status.
func (status ClusterAutoscalerStatus) GetReadableString() string {
	var buffer bytes.Buffer
	buffer.WriteString("Cluster-wide:\n")
	buffer.WriteString(getConditionsString(status.ClusterwideConditions, "  "))
	if len(status.NodeGroupStatuses) == 0 {
		return buffer.String()
	}
	buffer.WriteString("\nNodeGroups:\n")
	for _, nodeGroupStatus := range status.NodeGroupStatuses {
		buffer.WriteString(fmt.Sprintf("  Name:        %v\n", nodeGroupStatus.ProviderID))
		buffer.WriteString(getConditionsString(nodeGroupStatus.Conditions, "  "))
		buffer.WriteString("\n")
	}
	return buffer.String()
}

// LogItem is a single entry in log managed by LogCollector.
type LogItem struct {
	// Log is the logged message body.
	Log string
	// Timestamp when the Log was created.
	Timestamp time.Time
}

// LogCollector keeps recent log events. It is automatically truncated on each access based on predefined set of conditions.
type LogCollector struct {
	sync.Mutex
	// MaxItems is the maximum size of the log. Adding further messages will cause the oldest ones to be dropped.
	MaxItems int
	// ItemLifetime is the maximum time an item can be stored in log.
	ItemLifetime time.Duration
	// Store keeps LogItems.
	Store []LogItem
}

// NewLogCollector creates new LogCollector.
func NewLogCollector() *LogCollector {
	return &LogCollector{
		MaxItems:     DefaultLogCollectorMaxItems,
		ItemLifetime: DefaultLogCollectorItemLifetime,
		Store:        make([]LogItem, 0)}
}

// To be executed under a lock.
func (lc *LogCollector) compact(now time.Time) {
	firstIndex := 0
	if len(lc.Store) > lc.MaxItems {
		firstIndex = len(lc.Store) - lc.MaxItems
	}
	for ; firstIndex < len(lc.Store); firstIndex++ {
		lifetimeEnd := lc.Store[firstIndex].Timestamp.Add(lc.ItemLifetime)
		if now.Before(lifetimeEnd) {
			break
		}
	}
	if firstIndex > 0 {
		lc.Store = append(lc.Store[:0], lc.Store[firstIndex:]...)
	}
}

// Log logs a single provided message in LogCollector.
func (lc *LogCollector) Log(msg string, timestamp time.Time) {
	lc.Lock()
	defer lc.Unlock()
	lc.Store = append(lc.Store, LogItem{Log: msg, Timestamp: timestamp})
	lc.compact(timestamp)
}

// GetLogs returns a copy of messages in log. This is an actual copy, so it will not reflect any future changes in log.
func (lc *LogCollector) GetLogs(now time.Time) []LogItem {
	lc.Lock()
	defer lc.Unlock()
	lc.compact(now)
	result := make([]LogItem, len(lc.Store))
	copy(result, lc.Store)
	return result
}
