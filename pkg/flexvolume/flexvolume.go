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

package flexvolume

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Driver interface {
	Init() (map[string]interface{}, error)
	Attach(jsonOptions, nodeName string) (map[string]interface{}, error)
	Detach(mountDev, nodeName string) (map[string]interface{}, error)
	WaitForAttach(mountDev, jsonOptions string) (map[string]interface{}, error)
	IsAttached(jsonOptions, nodeName string) (map[string]interface{}, error)
	Mount(targetMountDir, jsonOptions string) (map[string]interface{}, error)
	Unmount(targetMountDir string) (map[string]interface{}, error)
}

type driverOp func(Driver, []string) (map[string]interface{}, error)

type cmdInfo struct {
	numArgs int
	run     driverOp
}

type FlexVolume struct {
	Driver
	commands map[string]cmdInfo
}

func NewFlexVolume(d Driver) *FlexVolume {
	return &FlexVolume{
		Driver: d,
		commands: map[string]cmdInfo{
			"init": {
				0, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.Init()
				},
			},
			"attach": {
				2, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.Attach(args[0], args[1])
				},
			},
			"detach": {
				2, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.Detach(args[0], args[1])
				},
			},
			"waitforattach": {
				2, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.WaitForAttach(args[0], args[1])
				},
			},
			"isattached": {
				2, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.IsAttached(args[0], args[1])
				},
			},
			"mount": {
				2, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.Mount(args[0], args[1])
				},
			},
			"unmount": {
				1, func(d Driver, args []string) (map[string]interface{}, error) {
					return d.Unmount(args[0])
				},
			},
		},
	}
}

func (f *FlexVolume) doRun(args []string) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments passed to flexvolume driver")
	}
	nArgs := len(args) - 1
	op := args[0]
	if cmdInfo, found := f.commands[op]; found {
		if cmdInfo.numArgs == nArgs {
			return cmdInfo.run(f, args[1:])
		} else {
			return nil, fmt.Errorf("unexpected number of args %d (expected %d) for operation %q", nArgs, cmdInfo.numArgs, op)
		}
	} else {
		return map[string]interface{}{
			"status": "Not supported",
		}, nil
	}
}

func (d *FlexVolume) Run(args []string) string {
	return formatResult(d.doRun(args))
}

func formatResult(fields map[string]interface{}, err error) string {
	var data map[string]interface{}
	if err != nil {
		data = map[string]interface{}{
			"status":  "Failure",
			"message": err.Error(),
		}
	} else {
		data = map[string]interface{}{
			"status": "Success",
		}
		for k, v := range fields {
			data[k] = v
		}
	}
	s, err := json.Marshal(data)
	if err != nil {
		panic("error marshalling the data")
	}
	return string(s) + "\n"
}
