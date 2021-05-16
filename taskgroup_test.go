package taskgroup

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var TOTAL = 1000 * 100
var expectedError = fmt.Errorf("expected error")

func delay(n int) (int, error) {
	time.Sleep(time.Duration(n) * time.Millisecond)
	return n, nil
}

func loop(fn func()) {
	pg := New(&Option{Limit: 1000})
	go func() {
		for i := 0; i < TOTAL; i++ {
			pg.Go(func() (data interface{}, err error) {
				fn()
				return
			})
		}
		pg.Fed()
	}()

	for range pg.Result() {
	}
}

// taskgroup basic test
func TestBasic(t *testing.T) {

	loop(func() {

		var ch = make(chan int, 5)

		worker := func(ctx context.Context, n int) {
			go func() {
				<-ctx.Done()
				ch <- n
			}()
		}

		arr := []int{5, 4, 3, 2, 1}
		newArr := []int{}

		defer func() {
			// since panic, taskgroup deferred cancel will be called.
			// newArr will be [1,2,3,0,-10].
			assert.Equal(t, 5, len(newArr))

			ret := 0
			for _, v := range newArr {
				ret += v
			}
			assert.Equal(t, 1, ret)

			sum := 0
			sum1 := 0
			for _, v := range arr {
				n := <-ch
				sum = sum + n
				sum1 = sum1 + v
			}
			assert.Equal(t, sum1, sum)
		}()

		g, ctx := WithContext(context.Background(), &Option{Limit: 5})
		defer g.Cancel()

		for _, v := range arr {
			v := v
			g.Go(func() (interface{}, error) {
				worker(ctx, v)

				_, err := delay(v * v * 10)

				if v == 5 {
					panic("unexpected error")
				}
				if v == 4 {
					return 0, expectedError
				}

				return v, err
			})
		}

		// tasks fed
		g.Fed()

		i := 0
		for p := range g.Result() {
			err := p.Err

			if err != nil {
				if err == expectedError {
					// ignore expectedError.
					newArr = append(newArr, 0)
					continue
				}
				// panic will be recovered by taskgroup, and return a error.
				assert.Contains(t, err.Error(), "taskgroup: panic recovered:")
				newArr = append(newArr, -5)
				continue
			}

			v, ok := (p.Data).(int)
			if !ok {
				fmt.Printf("TestBasic p.Data error\n\n")
				fmt.Printf("v: %d -- i: %d\n\n", v, i)
				return
			}

			// v will be 1,2,3.
			// since huge concurrent, the order could be not strong
			// assert.Equal(t, i+1, v)
			newArr = append(newArr, v)
			i++
		}
	})

}

// taskgroup limit test
func TestLimit(t *testing.T) {

	loop(func() {

		arr := []int{5, 4, 3, 2, 1}
		g := New(&Option{Limit: 1})
		defer g.Cancel()

		// the loop need be wrapped by a goroutine when the limit less than your tasks count,
		// else dead lock will be created.
		go func() {
			for _, v := range arr {
				v := v
				g.Go(func() (interface{}, error) {
					return delay(v)
				})
			}

			// tasks fed
			g.Fed()
		}()

		i := 0
		for p := range g.Result() {
			v, ok := (p.Data).(int)
			if !ok {
				fmt.Printf("TestLimit p.Data error\n\n")
				return
			}
			// v will be 5, 4, 3, 2, 1.
			// since taskgroup limit is 1, so tasks will be executed one by one.
			assert.Equal(t, arr[i], v)
			i++
		}

	})
}

// taskgroup timeout test
func TestTimeout(t *testing.T) {

	loop(func() {

		// cancel after 1 millisecond later
		ctx, _ := context.WithTimeout(context.Background(), time.Millisecond)

		g, _ := WithContext(ctx, &Option{MaxErrorCount: 1})

		go func() {
			// go it right now
			g.Go(func() (interface{}, error) {
				return delay(0)
			})

			// wait 100 millisecond
			delay(100)

			// now is 1 millisecond later, the ctx has timed out
			// so it wiil never be called
			g.Go(func() (interface{}, error) {
				return delay(0)
			})

			// tasks fed
			g.Fed()
		}()

		count := 0
		for range g.Result() {
			count++
		}
		assert.LessOrEqual(t, count, 1)
	})
}

// taskgroup one error test
func TestError(t *testing.T) {

	loop(func() {

		n := 100
		g := New(&Option{MaxErrorCount: 1})

		for i := 0; i < n; i++ {
			g.Go(func() (interface{}, error) {
				delay(0)
				return nil, expectedError
			})
		}

		// tasks fed
		g.Fed()

		count := 0
		for p := range g.Result() {
			count++
			assert.Equal(t, p.Err, expectedError)
		}
		assert.Equal(t, 1, count)
	})
}

// taskgroup manual cancel test
func TestManualCancel(t *testing.T) {

	loop(func() {
		n := 100
		mid := 50
		g := New(&Option{MaxErrorCount: 1})

		for i := 0; i < n; i++ {
			i := i
			g.Go(func() (interface{}, error) {
				data, err := delay(0)
				if i == mid {
					// manual cancel
					g.Cancel()
				}
				return data, err
			})
		}

		// tasks fed
		g.Fed()
		count := 0
		for range g.Result() {
			count++
		}
		// since gracefully cancel, mid <= count
		assert.LessOrEqual(t, mid, count)
	})
}

// taskgroup random cancel test
func TestRandomCancel(t *testing.T) {

	loop(func() {
		n := 10
		ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*100)
		g, _ := WithContext(ctx, &Option{MaxErrorCount: 1})

		for i := 0; i < n; i++ {
			g.Go(func() (interface{}, error) {
				data, err := delay(rand.Intn(10))
				if rand.Intn(10) > 5 {
					// random cancel
					g.Cancel()
				}
				return data, err
			})
		}

		// tasks fed
		g.Fed()
		count := 0
		for range g.Result() {
			count++
		}
		// since gracefully cancel, count <= n
		assert.LessOrEqual(t, count, n)

	})
}

// taskgroup provider and consumer mode example
func TestProviderAndConsumer(t *testing.T) {

	loop(func() {

		g := provider()

		consumer(g)

		// some logic ...
		delay(10)

		// cancel provider and consumer
		g.Cancel()
	})
}

func provider() Group {
	g := New(&Option{MaxErrorCount: 1})

	go func() {
		for {
			select {
			case <-g.Ctx().Done():
				return
			default:
				g.Go(func() (interface{}, error) {
					return nil, nil
				})
			}
			delay(1)
		}
	}()

	return g
}

func consumer(g Group) {
	go func() {
		for range g.Result() {
			// it is just consuming all message from provider till provider want to stop, so
			// g.Fed() is not necessary
		}
	}()
}
