/*
Copyright 2016 The Kubernetes Authors.

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

package main

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/util/clock"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/wait"
)

var (
	fakePeriod = 2 * time.Second
)

func getFakeCmdTemplate() exec.FakeCmd {
	return exec.FakeCmd{
		CombinedOutputScript: []exec.FakeCombinedOutputAction{
			// Success.
			func() ([]byte, error) { return []byte{}, nil },
			// Failure.
			func() ([]byte, error) { return nil, &exec.FakeExitError{Status: 2} },
			// Success.
			func() ([]byte, error) { return []byte{}, nil },
		},
	}
}

func getFakeExecTemplate(fakeCmd *exec.FakeCmd) exec.FakeExec {
	return exec.FakeExec{
		CommandScript: []exec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return exec.InitFakeCmd(fakeCmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return exec.InitFakeCmd(fakeCmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return exec.InitFakeCmd(fakeCmd, cmd, args...) },
		},
	}
}

func TestSingleExecWorker(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	fakeCmd := getFakeCmdTemplate()
	fakeExec := getFakeExecTemplate(&fakeCmd)

	readyCh := make(chan struct{}, 1)
	fakeProber := newExecWorker("echo healthz", "/healthz", fakePeriod, &fakeExec, fakeClock, readyCh)
	defer close(fakeProber.stopCh)
	go fakeProber.start()

	t.Logf("Wait for fakeProber to be ready.")
	<-readyCh
	numOfCalls := 0

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExec, &numOfCalls, fakeProber, true); err != nil {
		t.Errorf("%v\n", err)
	}

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExec, &numOfCalls, fakeProber, false); err != nil {
		t.Errorf("%v\n", err)
	}

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExec, &numOfCalls, fakeProber, true); err != nil {
		t.Errorf("%v\n", err)
	}
}

func TestMultipleExecWorkers(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	fakeCmdOne := getFakeCmdTemplate()
	fakeExecOne := getFakeExecTemplate(&fakeCmdOne)
	fakeCmdTwo := getFakeCmdTemplate()
	fakeExecTwo := getFakeExecTemplate(&fakeCmdTwo)

	readyChOne := make(chan struct{}, 1)
	fakeProberOne := newExecWorker("echo healthz1", "/healthz1", fakePeriod, &fakeExecOne, fakeClock, readyChOne)
	readyChTwo := make(chan struct{}, 1)
	fakeProberTwo := newExecWorker("echo healthz2", "/healthz2", fakePeriod, &fakeExecTwo, fakeClock, readyChTwo)
	defer func() {
		close(fakeProberOne.stopCh)
		close(fakeProberTwo.stopCh)
	}()
	go fakeProberOne.start()
	go fakeProberTwo.start()

	t.Logf("Wait for fakeProbers to be ready.")
	<-readyChOne
	<-readyChTwo
	numOfCallsOne := 0
	numOfCallsTwo := 0

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExecOne, &numOfCallsOne, fakeProberOne, true); err != nil {
		t.Errorf("%v\n", err)
	}
	if err := waitForProberExec(t, &fakeExecTwo, &numOfCallsTwo, fakeProberTwo, true); err != nil {
		t.Errorf("%v\n", err)
	}

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExecOne, &numOfCallsOne, fakeProberOne, false); err != nil {
		t.Errorf("%v\n", err)
	}
	if err := waitForProberExec(t, &fakeExecTwo, &numOfCallsTwo, fakeProberTwo, false); err != nil {
		t.Errorf("%v\n", err)
	}

	fakeClock.Step(fakePeriod)
	if err := waitForProberExec(t, &fakeExecOne, &numOfCallsOne, fakeProberOne, true); err != nil {
		t.Errorf("%v\n", err)
	}
	if err := waitForProberExec(t, &fakeExecTwo, &numOfCallsTwo, fakeProberTwo, true); err != nil {
		t.Errorf("%v\n", err)
	}
}

func waitForProberExec(t *testing.T, fakeExec *exec.FakeExec, numOfCalls *int, prober *execWorker, noError bool) error {
	(*numOfCalls)++
	return wait.Poll(5*time.Millisecond, 5*time.Second, func() (done bool, err error) {
		if fakeExec.CommandCalls != *numOfCalls {
			t.Logf("unexpected number of command calls: %d, expected: %d", fakeExec.CommandCalls, *numOfCalls)
			return false, nil
		}
		err = prober.getResults().err
		if noError == true && err != nil {
			t.Logf("expected no error, got: %v\n", err)
			return false, nil
		} else if noError == false && err == nil {
			t.Logf("expected error, got no error")
			return false, nil
		}
		return true, nil
	})
}
