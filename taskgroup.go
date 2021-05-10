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
// group struct is private, A zero group is invalid, getting group must use Create.
type group struct {
	ret    chan *payload
	lmt    chan struct{}
	done   chan struct{}
	cancel func()

	workerOnce sync.Once
	cancelOnce sync.Once
}

// group result chanel payload.
type payload struct {
	Data interface{}
	Err  error
}

// Create returns a new Group.
func Create() *group {
	ret := make(chan *payload)
	done := make(chan struct{})

	return &group{ret: ret, done: done}
}

// Create returns a new Group and an associated Context derived from ctx.
func WithContext(ctx context.Context) (*group, context.Context) {
	g := Create()

	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel

	return g, ctx
}

func (g *group) Cancel() {
	g.cancelOnce.Do(func() {
		close(g.done)
		if g.cancel != nil {
			g.cancel()
		}
	})

}

// Returns the group result channel. You can for-range it, and then get data and err like following:
// 	for i := 0; i < count; i++ {
// 		p := <- g.Result
// 		if p.Err != nil {
// 			g.Cancel()
// 			return
// 		}
// 		...
//	}
//
// You can cancel the all subtasks of the group or ignore it, when the error occur.
func (g *group) Result() <-chan *payload {
	return g.ret
}

// Just alias.
type fn = func() (interface{}, error)

// Go calls the given function in a new goroutine, the goroutines number can be limited.
func (g *group) Go(f fn) {
	select {
	case <-g.done:
	default:
		if g.lmt != nil {
			g.lmt <- struct{}{}
		}
		go g.do(f)
	}
}

// It will execute your function, obtain results, and send them to channel.
func (g *group) do(f fn) {
	var (
		data interface{}
		err  error
	)

	defer func() {
		// Listening done signal to avoid goroutines leak.
		select {
		case <-g.done:
		default:
			if g.lmt != nil {
				<-g.lmt
			}
			g.ret <- &payload{data, err}
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			err = fmt.Errorf("taskgroup: panic recovered: %s\n%s", r, buf)
		}
	}()

	data, err = f()
}

// It is to limit goroutine workers.
func (g *group) Limit(n int) *group {
	if n <= 0 {
		panic("taskgroup: limit must great than 0")
	}
	g.workerOnce.Do(func() {
		g.lmt = make(chan struct{}, n)
	})
	return g
}
