// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stage0

import (
	"github.com/appc/spec/schema/types"
)

// Attach runs attach entrypoint, crossing the stage0/stage1 border.
func Attach(cdir string, podPID int, appName types.ACName, stage1Path string, uuid string, args []string) error {
	ce := CrossingEntrypoint{
		PodPath:        cdir,
		PodPID:         podPID,
		AppName:        appName.String(),
		EntrypointName: attachEntrypoint,
		EntrypointArgs: args,
		Interactive:    true,
	}

	if err := ce.Run(); err != nil {
		return err
	}

	return nil
}
