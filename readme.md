# Taskgroup

> A simple and useful goroutine concurrent library.


## Installation

```bash
go get github.com/anthhub/taskgroup
```


## Usage

```go
	count := 100
	// create a taskgroup
	// set max error count to 1
	g := New(&Option{MaxErrorCount:1})

	for i := 0; i < count; i++ {
		g.Go(func() (interface{}, error) {
			// your work function
			return worker()
		})
	}

	// declare the end of tasks producing
	g.Fed()

	// it will receive a message when a task of the group return an error or
	// till all tasks finish.
	if p := <-g.Result(); p.Err != nil {
		return
	}
```

## Advanced Usage

### Useful Options

```go
	count := 100
	// configure group timeout
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	// create a taskgroup with context
	g, ctx = WithContext(ctx, &Option{
		// limit the count of goroutine workers; default is infinity
		Limit: 5,
		// set max error count to 5; default is infinity
		MaxErrorCount: 5,
		// disable recover panic; default will recover panic
		DisableRecover: true,
	})

	// the loop need be wrapped by a goroutine when the limit less than your tasks count,
	// else dead lock will be created.
	go func() {
		for i := 0; i < count; i++ {
			g.Go(func() (interface{}, error) {
				// your work function
				// panic will be recover and return a error
				return worker(ctx)
			})
		}

		// it is to tell consumer that consuming the all tasks and then close itself.
		// 
		// it is very important else the consuming never end without g.Fed() when all 
		// tasks have finished.
		g.Fed()
	}()

	// you can directly for-range g.Result(), it will break the loop when all tasks id finished
	// or error count is to 5.
	for p := range g.Result() {
		if p.Err != nil {
			glog.Errorf("error")
		}
		// you can cancel the group and break the loop in advanced when you want.
		if [condition] {
			g.Cancel()
			break
		}
		data := p.Data
		// ...
	}
	// ...
```


### Producer & Consumer Mode

```go
	func main() {
		g := provider()

		consumer(g)

		// some logic ...
		delay(10)

		// cancel provider and consumer
		g.Cancel()
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
						// get group inner ctx
						return worker(g.Ctx())
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
```

> If you want to learn more about `taskgroup`, you can read test cases and source code.
