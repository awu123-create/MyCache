package Singleflight

import "sync"

// 表示一次对某个key的请求
type call struct {
	// 等待第一次发起请求的goroutine完成
	wg sync.WaitGroup
	// 请求结果
	val interface{}
	err error
}

// 管理所有key的call
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

// Do方法的作用：
// 1.如果key已经存在，则等待第一次请求完成并返回结果
// 2.如果key不存在，则创建一个新的call，并执行fn()
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
