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

// Package hyper contains HyperContainer implementation of runtime API.
// TODO: import this package from hyperhq/hyper directly when it move out of integration.

package hyper

import (
	"encoding/binary"
	"fmt"

	"github.com/docker/docker/pkg/stdcopy"
)

type StreamExtractor interface {
	Extract(orig []byte) ([]byte, []byte, error)
}

type RawExtractor struct{}

const (
	// Stdin represents standard input stream type.
	Stdin stdcopy.StdType = iota
	// Stdout represents standard output stream type.
	Stdout
	// Stderr represents standard error steam type.
	Stderr

	stdWriterPrefixLen = 8
	stdWriterFdIndex   = 0
	stdWriterSizeIndex = 4
)

type StdcopyExtractor struct {
	readingHead bool
	current     stdcopy.StdType
	remain      int

	headbuf []byte
	headlen int
}

func NewExtractor(tty bool) StreamExtractor {
	if tty {
		return &RawExtractor{}
	}
	return &StdcopyExtractor{
		readingHead: true,
		headbuf:     make([]byte, stdWriterPrefixLen),
	}
}

func (r *RawExtractor) Extract(orig []byte) ([]byte, []byte, error) {
	return orig, nil, nil
}

func (s *StdcopyExtractor) Extract(orig []byte) ([]byte, []byte, error) {
	var (
		stdout = []byte{}
		stderr = []byte{}
	)
	for len(orig) > 0 {
		if s.readingHead {
			hrl := stdWriterPrefixLen - s.headlen //hrl -- head remain length
			if len(orig) < hrl {
				copy(s.headbuf[s.headlen:], orig)
				s.headlen += len(orig)
				return stdout, stderr, nil
			}

			copy(s.headbuf[s.headlen:], orig[:hrl])
			orig = orig[hrl:]
			s.headlen = 0

			stype := stdcopy.StdType(s.headbuf[stdWriterFdIndex])
			if stype != Stdout && stype != Stderr {
				return stdout, stderr, fmt.Errorf("invalid stream type %x", stype)
			}

			s.current = stype
			s.remain = int(binary.BigEndian.Uint32(s.headbuf[stdWriterSizeIndex : stdWriterSizeIndex+4]))
			s.readingHead = false
		}

		var (
			msg []byte
			ml  int
		)
		if len(orig) < s.remain {
			s.remain -= len(orig)
			ml = len(orig)
		} else {
			ml = s.remain
			s.readingHead = true
			s.remain = 0
		}

		msg = orig[:ml]
		orig = orig[ml:]

		switch s.current {
		case Stdout:
			stdout = append(stdout, msg...)
		case Stderr:
			stderr = append(stderr, msg...)
		}
	}
	return stdout, stderr, nil

}
