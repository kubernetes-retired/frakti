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

package libvirt

import (
	"errors"
)

// NOTE: Keep same with github.com/libvirt/libvirt-go/domain.go
const (
	// DOMAIN_NOSTATE represents the domain is in no state
	DOMAIN_NOSTATE DomainState = iota
	// DOMAIN_RUNNING represents that the domain is running
	DOMAIN_RUNNING
	// DOMAIN_BLOCKED represents that the domain is blocked
	DOMAIN_BLOCKED
	// DOMAIN_PAUSED represents that the domain is paused
	DOMAIN_PAUSED
	// DOMAIN_SHUTDOWN represents that the domain is being shut down
	DOMAIN_SHUTDOWN
	// DOMAIN_CRASHED represents that the domain is crashed
	DOMAIN_CRASHED
	// DOMAIN_PMSUSPENDED represents that the domain is suspended
	DOMAIN_PMSUSPENDED
	// DOMAIN_SHUTOFF represents that the domain is shut off
	DOMAIN_SHUTOFF
)

// DomainState represents a state of a libvirt domain
type DomainState int

// ErrDomainNotFound err means can't find specificed domain
// returned by domain loop up methods
var ErrDomainNotFound = errors.New("domain not found")
