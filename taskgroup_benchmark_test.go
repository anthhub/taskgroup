package taskgroup

import (
	"testing"
)

func BenchmarkTaskgroup(b *testing.B) {
	l := int(b.N / 10)
	if l <= 0 {
		l = 2
	}
	g := Create()
	g.Limit(l)
	defer g.Cancel()

	b.StartTimer()

	go func() {
		for i := 0; i < b.N; i++ {
			g.Go(func() (interface{}, error) {
				return consumer(0)
			})
		}
	}()

	for i := 0; i < b.N; i++ {
		p := <-g.Result()
		err := p.Err
		err = err
		n := (p.Data).(int)
		n = n
	}
	b.StopTimer()
}

func BenchmarkChannel(b *testing.B) {
	type carry struct {
		Data int
		Err  error
	}

	l := int(b.N / 10)
	if l <= 0 {
		l = 2
	}
	ch := make(chan *carry)
	limit := make(chan struct{}, l)

	b.StartTimer()
	go func() {
		for i := 0; i < b.N; i++ {
			limit <- struct{}{}
			go func() {
				var (
					data int
					err  error
				)
				defer func() {
					<-limit
					ch <- &carry{data, err}

				}()
				data, err = consumer(0)
			}()
		}
	}()

	for i := 0; i < b.N; i++ {
		p := <-ch
		err := p.Err
		err = err
		n := p.Data
		n = n
	}
	b.StopTimer()
}

func BenchmarkTaskgroupWithoutLimit(b *testing.B) {
	g := Create()
	defer g.Cancel()

	b.StartTimer()
	go func() {
		for i := 0; i < b.N; i++ {
			g.Go(func() (interface{}, error) {
				return consumer(0)
			})
		}
	}()

	for i := 0; i < b.N; i++ {
		p := <-g.Result()
		err := p.Err
		err = err
		n := (p.Data).(int)
		n = n
	}
	b.StopTimer()
}

func BenchmarkChannelWithoutLimit(b *testing.B) {
	type carry struct {
		Data int
		Err  error
	}

	ch := make(chan *carry)
	b.StartTimer()
	go func() {
		for i := 0; i < b.N; i++ {
			go func() {
				var (
					data int
					err  error
				)
				defer func() {
					ch <- &carry{data, err}
				}()
				data, err = consumer(0)
			}()
		}
	}()

	for i := 0; i < b.N; i++ {
		p := <-ch
		err := p.Err
		err = err
		n := p.Data
		n = n
	}
	b.StopTimer()
}
