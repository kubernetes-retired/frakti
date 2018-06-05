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

package proc

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// IO is used for containerd-kata plugin
type IO interface {
	io.Closer
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Set(*exec.Cmd)
}

// StartCloser starts to close
type StartCloser interface {
	CloseAfterStart() error
}

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32<<10)
		return &buffer
	},
}

// NewPipeIO creates pipe pairs to be used with kata containers
func NewPipeIO(uid, gid int) (i IO, err error) {
	var pipes []*pipe
	// cleanup in case of an error
	defer func() {
		if err != nil {
			for _, p := range pipes {
				p.Close()
			}
		}
	}()
	stdin, err := newPipe()
	if err != nil {
		return nil, err
	}
	pipes = append(pipes, stdin)
	if err = unix.Fchown(int(stdin.r.Fd()), uid, gid); err != nil {
		return nil, errors.Wrap(err, "failed to chown stdin")
	}

	stdout, err := newPipe()
	if err != nil {
		return nil, err
	}
	pipes = append(pipes, stdout)
	if err = unix.Fchown(int(stdout.w.Fd()), uid, gid); err != nil {
		return nil, errors.Wrap(err, "failed to chown stdout")
	}

	stderr, err := newPipe()
	if err != nil {
		return nil, err
	}
	pipes = append(pipes, stderr)
	if err = unix.Fchown(int(stderr.w.Fd()), uid, gid); err != nil {
		return nil, errors.Wrap(err, "failed to chown stderr")
	}

	return &pipeIO{
		in:  stdin,
		out: stdout,
		err: stderr,
	}, nil
}

func newPipe() (*pipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &pipe{
		r: r,
		w: w,
	}, nil
}

type pipe struct {
	r *os.File
	w *os.File
}

func (p *pipe) Close() error {
	err := p.r.Close()
	if werr := p.w.Close(); err == nil {
		err = werr
	}
	return err
}

type pipeIO struct {
	in  *pipe
	out *pipe
	err *pipe
}

func (i *pipeIO) Stdin() io.WriteCloser {
	return i.in.w
}

func (i *pipeIO) Stdout() io.ReadCloser {
	return i.out.r
}

func (i *pipeIO) Stderr() io.ReadCloser {
	return i.err.r
}

func (i *pipeIO) Close() error {
	var err error
	for _, v := range []*pipe{
		i.in,
		i.out,
		i.err,
	} {
		if cerr := v.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

func (i *pipeIO) CloseAfterStart() error {
	for _, f := range []*os.File{
		i.out.w,
		i.err.w,
	} {
		f.Close()
	}
	return nil
}

// Set sets the io to the exec.Cmd
func (i *pipeIO) Set(cmd *exec.Cmd) {
	cmd.Stdin = i.in.r
	cmd.Stdout = i.out.w
	cmd.Stderr = i.err.w
}

// NewSTDIO new a standard IO
func NewSTDIO() (IO, error) {
	return &stdio{}, nil
}

type stdio struct {
}

func (s *stdio) Close() error {
	return nil
}

func (s *stdio) Set(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}

func (s *stdio) Stdin() io.WriteCloser {
	return os.Stdin
}

func (s *stdio) Stdout() io.ReadCloser {
	return os.Stdout
}

func (s *stdio) Stderr() io.ReadCloser {
	return os.Stderr
}

// NewNullIO returns IO setup for /dev/null use with kata containers
func NewNullIO() (IO, error) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	return &nullIO{
		devNull: f,
	}, nil
}

type nullIO struct {
	devNull *os.File
}

func (n *nullIO) Close() error {
	// this should be closed after start but if not
	// make sure we close the file but don't return the error
	n.devNull.Close()
	return nil
}

func (n *nullIO) Stdin() io.WriteCloser {
	return nil
}

func (n *nullIO) Stdout() io.ReadCloser {
	return nil
}

func (n *nullIO) Stderr() io.ReadCloser {
	return nil
}

func (n *nullIO) Set(c *exec.Cmd) {
	// don't set STDIN here
	c.Stdout = n.devNull
	c.Stderr = n.devNull
}

func (n *nullIO) CloseAfterStart() error {
	return n.devNull.Close()
}

func copyPipes(ctx context.Context, rio IO, stdin, stdout, stderr string, wg, cwg *sync.WaitGroup) error {
	var sameFile io.WriteCloser
	for _, i := range []struct {
		name string
		dest func(wc io.WriteCloser, rc io.Closer)
	}{
		{
			name: stdout,
			dest: func(wc io.WriteCloser, rc io.Closer) {
				wg.Add(1)
				cwg.Add(1)
				go func() {
					cwg.Done()
					p := bufPool.Get().(*[]byte)
					defer bufPool.Put(p)
					io.CopyBuffer(wc, rio.Stdout(), *p)
					wg.Done()
					wc.Close()
					if rc != nil {
						rc.Close()
					}
				}()
			},
		}, {
			name: stderr,
			dest: func(wc io.WriteCloser, rc io.Closer) {
				wg.Add(1)
				cwg.Add(1)
				go func() {
					cwg.Done()
					p := bufPool.Get().(*[]byte)
					defer bufPool.Put(p)
					io.CopyBuffer(wc, rio.Stderr(), *p)
					wg.Done()
					wc.Close()
					if rc != nil {
						rc.Close()
					}
				}()
			},
		},
	} {
		ok, err := isFifo(i.name)
		if err != nil {
			return err
		}
		var (
			fw io.WriteCloser
			fr io.Closer
		)
		if ok {
			if fw, err = fifo.OpenFifo(ctx, i.name, syscall.O_WRONLY, 0); err != nil {
				return fmt.Errorf("opening %s failed: %s", i.name, err)
			}
			if fr, err = fifo.OpenFifo(ctx, i.name, syscall.O_RDONLY, 0); err != nil {
				return fmt.Errorf("opening %s failed: %s", i.name, err)
			}
		} else {
			if sameFile != nil {
				i.dest(sameFile, nil)
				continue
			}
			if fw, err = os.OpenFile(i.name, syscall.O_WRONLY|syscall.O_APPEND, 0); err != nil {
				return fmt.Errorf("opening %s failed: %s", i.name, err)
			}
			if stdout == stderr {
				sameFile = fw
			}
		}
		i.dest(fw, fr)
	}
	if stdin == "" {
		rio.Stdin().Close()
		return nil
	}
	f, err := fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("containerd-shim: opening %s failed: %s", stdin, err)
	}
	cwg.Add(1)
	go func() {
		cwg.Done()
		p := bufPool.Get().(*[]byte)
		defer bufPool.Put(p)

		io.CopyBuffer(rio.Stdin(), f, *p)
		rio.Stdin().Close()
		f.Close()
	}()
	return nil
}

// isFifo checks if a file is a fifo
// if the file does not exist then it returns false
func isFifo(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if stat.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
		return true, nil
	}
	return false, nil
}
