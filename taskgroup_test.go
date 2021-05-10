package taskgroup

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var globalMap = make(map[int]bool, 5)
var lck sync.Mutex

func worker(ctx context.Context, n int) {
	go func() {
		<-ctx.Done()
		lck.Lock()
		defer lck.Unlock()
		globalMap[n] = true
	}()
}

func consumer(n int) (int, error) {
	time.Sleep(time.Duration(n*10) * time.Millisecond)
	return n, nil
}

// taskgroup basic test
func TestBasic(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	expectError := fmt.Errorf("expected error")
	arr := []int{5, 4, 3, 2, 1}
	newArr := []int{}

	defer func() {
		// cause panic, taskgroup deferred cancel will be called.
		// newArr will be [1,2,3,0].
		assert.Equal(t, len(newArr), 4)

		time.Sleep(10 * time.Millisecond)
		// taskgroup cancel will terminal all workers.
		// globalMap will be { 1: true, 2: true,  3: true,  4: true,  5: true}.
		assert.Equal(t, len(globalMap), 5)
		for _, v := range arr {
			assert.Equal(t, globalMap[v], true)
		}

	}()

	g, ctx := WithContext(context.Background())
	g.Limit(5)
	defer g.Cancel()

	for _, v := range arr {
		v := v
		g.Go(func() (interface{}, error) {
			worker(ctx, v)

			res, err := consumer(v)

			if res == 5 {
				panic("unexpected error")
			}
			if res == 4 {
				return 0, expectError
			}

			return res, err
		})
	}

	for i := range arr {
		p := <-g.Result()
		err := p.Err

		if err != nil {
			if err == expectError {
				// ignore expectError.
				newArr = append(newArr, 0)
				continue
			}
			// panic will be recovered by taskgroup, and return a error.
			assert.Contains(t, err.Error(), "taskgroup: panic recovered:")
			return
		}

		v, ok := (p.Data).(int)
		if !ok {

			fmt.Printf("TestBasic p.Data error\n\n")
			fmt.Printf("i: %d\n\n", i)
			return
		}

		// v will be 1,2,3.
		assert.Equal(t, i+1, v)
		newArr = append(newArr, v)
	}
}

// taskgroup limit test
func TestLimit(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	arr := []int{5, 4, 3, 2, 1}
	g := Create().Limit(1)
	defer g.Cancel()

	// the loop need be wrapped by a goroutine when the limit less than your tasks count,
	// else dead lock will be created.
	go func() {
		for _, v := range arr {
			v := v
			g.Go(func() (interface{}, error) {
				return consumer(v)
			})
		}
	}()

	for i := range arr {
		p := <-g.Result()
		v, ok := (p.Data).(int)
		if !ok {
			fmt.Printf("TestLimit p.Data error\n\n")
			return
		}
		// v will be 5, 4, 3, 2, 1.
		// cause taskgroup limit is 1, so tasks will be executed one by one.
		assert.Equal(t, arr[i], v)
	}
}

// taskgroup manual cancel test
func TestManualCancel(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	count := 100
	g := Create()

	go func() {
		for i := 0; i < count; i++ {
			v := rand.Intn(5)
			g.Go(func() (interface{}, error) {
				return consumer(v)
			})
		}
	}()

	for i := 0; i < count; i++ {
		<-g.Result()
		if i > 10 {
			g.Cancel()
			return
		}
	}
}

func TestManualCancelWithLimit(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	count := 100
	g := Create().Limit(2)

	go func() {
		for i := 0; i < count; i++ {
			v := rand.Intn(5)
			g.Go(func() (interface{}, error) {
				return consumer(v)
			})
		}
	}()

	for i := 0; i < count; i++ {
		<-g.Result()
		if i > 10 {
			g.Cancel()
			return
		}
	}
}

func TestManualCancelImmediately(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	count := 100
	g := Create()

	go func() {
		for i := 0; i < count; i++ {
			v := rand.Intn(5)
			g.Go(func() (interface{}, error) {
				return consumer(v)
			})
		}
	}()

	g.Cancel()
}

func TestManualCancelWithLimitImmediately(t *testing.T) {

	defer func() {
		consumer(10)
		// no goroutines leak.
		assert.Equal(t, 2, runtime.NumGoroutine())
	}()

	count := 100
	g := Create().Limit(10)

	go func() {
		for i := 0; i < count; i++ {
			v := rand.Intn(5)
			g.Go(func() (interface{}, error) {
				return consumer(v)
			})
		}
	}()

	g.Cancel()
}
