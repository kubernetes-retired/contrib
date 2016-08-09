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

package utils

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/wait"
)

// ReadPID returns the pid from the content of a pid file
func ReadPID(file string) (int, error) {
	pid, err := ioutil.ReadFile(file)
	if err != nil {
		return 0, err
	}
	pidInt, err := strconv.Atoi(strings.Trim(string(pid), "\n"))
	return pidInt, nil
}

// MonitorProcess monitor that the process identified by the pid in the given file is running.
// When the process stops or exits, it will send a signal on the given channel.
func MonitorProcess(pidFile string, exitChan chan struct{}) {
	// Wait for PID file to be constructed
	var pid int
	err := wait.PollImmediate(1*time.Second, time.Minute, func() (done bool, err error) {
		pid, err = ReadPID(pidFile)
		if err != nil {
			glog.Warningf("Error reading PID file %v: %v. Trying again...", pidFile, err)
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		glog.Errorf("Waited one minute for PID file %v to create", pidFile)
	} else {
		glog.Infof("Monitoring proccess %v. PID: %v", pidFile, pid)
		process, _ := os.FindProcess(pid)
		status, _ := process.Wait()
		glog.Errorf("Process %v has quit. Status %v.", pidFile, status)
	}
	close(exitChan)
}
