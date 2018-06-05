/*
Copyright 2018 The Kubernetes Authors.

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

package kata

import (
	"errors"
	"os"
	"sync"
	"syscall"
)

type pidPool struct {
	sync.Mutex
	pool map[uint32]struct{}
	cur  uint32
}

func newPidPool() *pidPool {
	return &pidPool{
		pool: make(map[uint32]struct{}),
		cur:  uint32(os.Getpid()),
	}
}

func (p *pidPool) Get() (uint32, error) {
	p.Lock()
	defer p.Unlock()

	pid := p.cur + 1
	// 32767 is the max pid of most 32bit Linux System. Maybe we can use other way to acquire the value.
	for pid != 32767 {
		process, err := os.FindProcess(int(pid))
		// flag indicates whether we can use this pid.
		flag := false
		if err != nil {
			// If the corresponding process is not found, it means that we can use this pid.
			flag = true
		} else {
			err := process.Signal(syscall.Signal(0))
			if err.Error() == "no such process" || err.Error() == "os: process already finished" {
				flag = true
			}
		}

		if flag {
			p.pool[pid] = struct{}{}
			return pid, nil
		}
		pid++
	}

	return 0, errors.New("pid pool exhausted")
}

func (p *pidPool) Put(pid uint32) {
	p.Lock()
	delete(p.pool, pid)
	p.Unlock()
}
