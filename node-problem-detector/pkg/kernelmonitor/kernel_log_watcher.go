/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kernelmonitor

import (
	"bufio"
	"os"

	"k8s.io/contrib/node-problem-detector/pkg/kernelmonitor/translator"
	"k8s.io/contrib/node-problem-detector/pkg/kernelmonitor/types"
	"k8s.io/contrib/node-problem-detector/pkg/kernelmonitor/util"

	"github.com/golang/glog"
	"github.com/hpcloud/tail"
)

const (
	defaultKernelLogPath = "/var/log/kern.log"
)

// WatcherConfig is the configuration of kernel log watcher.
type WatcherConfig struct {
	KernelLogPath string `json:"logPath, omitempty"`
}

// KernelLogWatcher watches and translates the kernel log. Once there is new log line,
// it will translate and report the log.
type KernelLogWatcher interface {
	// Watch starts the kernel log watcher and returns a watch channel.
	Watch() (<-chan *types.KernelLog, error)
	// Stop stops the kernel log watcher.
	Stop()
}

type kernelLogWatcher struct {
	// trans is the translator translates the log into internal format.
	trans translator.Translator
	cfg   WatcherConfig
	tl    *tail.Tail
	logCh chan *types.KernelLog
	tomb  *util.Tomb
}

// NewKernelLogWatcher creates a new kernel log watcher.
func NewKernelLogWatcher(cfg WatcherConfig) KernelLogWatcher {
	return &kernelLogWatcher{
		trans: translator.NewDefaultTranslator(),
		cfg:   cfg,
		tomb:  util.NewTomb(),
		// A capacity 1000 buffer should be enough
		logCh: make(chan *types.KernelLog, 1000),
	}
}

func (k *kernelLogWatcher) Watch() (<-chan *types.KernelLog, error) {
	path := defaultKernelLogPath
	if k.cfg.KernelLogPath != "" {
		path = k.cfg.KernelLogPath
	}
	start, err := k.getStartPoint(path)
	if err != nil {
		return nil, err
	}
	// TODO(random-liu): If the file gets recreated during this interval, the logic
	// will go wrong here.
	// TODO(random-liu): Rate limit tail file.
	// TODO(random-liu): Figure out what happens if log lines are removed.
	k.tl, err = tail.TailFile(path, tail.Config{
		Location: &tail.SeekInfo{
			Offset: start,
			Whence: os.SEEK_SET,
		},
		ReOpen: true,
		Follow: true,
	})
	if err != nil {
		return nil, err
	}
	glog.Info("Start watching kernel log")
	go k.watchLoop()
	return k.logCh, nil
}

func (k *kernelLogWatcher) Stop() {
	k.tomb.Stop()
}

// watchLoop is the main watch loop of kernel log watcher.
func (k *kernelLogWatcher) watchLoop() {
	defer func() {
		close(k.logCh)
		k.tomb.Done()
	}()
	for {
		select {
		case line := <-k.tl.Lines:
			// Notice that tail has trimmed '\n'
			if line.Err != nil {
				glog.Errorf("Tail error: %v", line.Err)
				continue
			}
			log, err := k.trans.Translate(line.Text)
			if err != nil {
				glog.Infof("Unable to parse line: %q, %v", line, err)
				continue
			}
			k.logCh <- log
		case <-k.tomb.Stopping():
			k.tl.Stop()
			glog.Infof("Stop watching kernel log")
			return
		}
	}
}

// getStartPoint parses the newest kernel log file and try to find the latest reboot point.
// Currently we rely on the kernel log timestamp to find the reboot point. The basic idea
// is straight forward: In the whole lifecycle of a node, the kernel log timestamp should
// always increase, only when it is reboot, the timestamp will decrease. We just parse the
// log and find the latest timestamp decreasing, then it should be the latest reboot point.
// TODO(random-liu): A drawback is that if the node is started long time ago, we'll only get
// logs in the newest kernel log file. We may want to improve this in the future.
func (k *kernelLogWatcher) getStartPoint(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	defer f.Close()
	start := int64(0)
	total := 0
	lastTimestamp := int64(0)
	reader := bufio.NewReader(f)
	done := false
	for !done {
		line, err := reader.ReadString('\n')
		if err != nil {
			done = true
		}
		total += len(line)
		log, err := k.trans.Translate(line)
		if err != nil {
			glog.Infof("unable to parse line: %q, %v", line, err)
			continue
		}
		if log.Timestamp < lastTimestamp {
			start = int64(total - len(line))
		}
		lastTimestamp = log.Timestamp
	}
	return start, nil
}
