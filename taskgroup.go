// Copyright 2021 The anthhub. All rights reserved.
// Refer to errgroup

// Package taskgroup provides synchronization, result and error propagation, max goroutines
// limit, panic recover and Context customized cancelation for groups of goroutines working
// on subtasks of a common task.
package taskgroup

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// A group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// The group struct is private, A zero group is invalid, getting a group must use New function.
type group struct {
	lock      sync.Mutex
	taskWg    sync.WaitGroup
	closeOnce sync.Once

	ret    chan *Payload
	ctx    context.Context
	cancel func()

	lmt    chan struct{}
	dr     bool
	amount uint32
	mec    uint32

	fulled  bool
	closing bool
}

// The group result struct.
type Payload struct {
	Data interface{}
	Err  error
}

// The group configuration option
type Option struct {
	Limit          uint32
	MaxErrorCount  uint32
	DisableRecover bool
}

// Just alias.
type Fn = func() (interface{}, error)

// Group interface
type Group interface {
	Cancel()
	Result() <-chan *Payload
	Ctx() context.Context
	Go(f Fn)
	Fed()
}

// It returns a new Group by options.
func New(options ...*Option) Group {
	g, _ := WithContext(context.Background(), options...)
	return g
}

// It returns a new Group and an associated Context derived from ctx.
func WithContext(ctx context.Context, options ...*Option) (Group, context.Context) {
	ret := make(chan *Payload)
	ctxWithCancel, cancel := context.WithCancel(ctx)

	g := &group{ret: ret, ctx: ctxWithCancel, cancel: cancel}
	g.config(options)

	go func() {
		// listening ctx done signal
		<-g.ctx.Done()
		g.gracefulClose()
	}()

	return g, ctxWithCancel
}

// It is to say the group is fulled, no more task will be sent, otherwise panic will occur!!!
//
// It is to tell the consumer that consumes the all tasks has sent, and then close itself.
//
// It is very important that call the Fed method when you want to stop tasks producing of
// the producer; it can terminate the consumer when all of the tasks are finished; otherwise
// consumer will always wait for more tasks to consuming!!!
func (g *group) Fed() {
	g.fulled = true

	go func() {
		// listening all of task finish
		g.taskWg.Wait()
		g.cancel()
		g.close()
	}()
}

// It can gracefully cancel all tasks of the group.
func (g *group) Cancel() {
	g.gracefulClose()
}

// It is to gracefully cancel tasks and close ret channel.
func (g *group) gracefulClose() {
	g.closing = true
	g.cancel()
}

// It closes the channel.
//
//  The four way to close ret channel:
// - all of tasks finish: close channel right now ( the premise is you called g.Fed()!!! )
// - manual cancel: must gracefully close
// - ctx is done: must gracefully close
// - meet error max count : must gracefully close
func (g *group) close() {
	g.closeOnce.Do(func() {
		close(g.ret)
	})
}

// It roles a consumer for finished tasks.
//
// It is to return the group result channel. You can for-range it and get data and err like following:
//
// 	for p := range g.Result {
// 		if p.Err != nil {
// 			g.Cancel()
// 			return
// 		}
// 		...
//	}
//
// You can cancel the all subtasks of the group or ignore it, when the error occur.
func (g *group) Result() <-chan *Payload {
	return g.ret
}

// It is to return the ctx of the group inner.
func (g *group) Ctx() context.Context {
	return g.ctx
}

// It is a producer to produce tasks for the group.
//
// Go method calls the given function in a new goroutine, the goroutines number can be limited.
//
// If you call it after calling Fed method, task will be rejected and panic will occur!!!
func (g *group) Go(f Fn) {
	if g.fulled {
		panic("taskgroup is fulled, more task will be rejected.")
	}

	if g.closing {
		return
	}

	g.taskWg.Add(1)

	if g.lmt != nil {
		g.lmt <- struct{}{}
	}

	go g.do(f)
}

// It will execute your function, obtain results, and send them to channel.
func (g *group) do(f Fn) {
	var (
		data interface{}
		err  error
	)

	defer func() {
		defer func() {
			if g.lmt != nil {
				<-g.lmt
			}
		}()

		g.handle(data, err)
	}()

	if g.dr {
		data, err = f()
		return
	}

	fn := func() (data interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 64<<10)
				buf = buf[:runtime.Stack(buf, false)]
				err = fmt.Errorf("taskgroup: panic recovered: %s\n%s", r, buf)
			}
		}()
		return f()
	}
	data, err = fn()
}

// It is to send the payload to result channel.
func (g *group) handle(data interface{}, err error) {
	defer g.taskWg.Done()

	if err != nil && g.mec > 0 {
		if g.amount >= g.mec {
			return
		}

		g.lock.Lock()
		g.amount++
		if g.amount == g.mec {
			g.ret <- &Payload{data, err}
			g.gracefulClose()
			g.lock.Unlock()
			return
		}
		g.lock.Unlock()

		if g.amount > g.mec {
			return
		}
	}

	g.ret <- &Payload{data, err}

}

// It is to configure group.
func (g *group) config(options []*Option) {
	option := &Option{}
	// merge options
	for _, o := range options {
		if o.DisableRecover {
			g.dr = o.DisableRecover
		}
		if o.MaxErrorCount > 0 {
			g.mec = o.MaxErrorCount
		}
		if o.Limit > 0 {
			option.Limit = o.Limit
		}
	}

	if option.Limit > 0 {
		g.lmt = make(chan struct{}, option.Limit)
	}
}
