package singleflight

import "sync"

// call 代表正在进行中，或已经结束的请求
type call struct {
	wg  sync.WaitGroup // 用于阻塞后面进来的并发请求
	val interface{}    // 函数返回的结果（比如查到的缓存数据）
	err error          // 函数返回的错误
}

// Group 是 Single-flight 的主类，管理不同 key 的请求(call)
type Group struct {
	mu sync.Mutex       // 保护 m 这个共享变量（你刚测过没锁会崩！）
	m  map[string]*call // 记录当前正在处理中的请求。key 是你要查的缓存键
}


// Do 接收一个 key 和一个用来获取数据的函数 fn（比如去 DB 查数据）
// 无论多少并发调用 Do，针对同一个 key，fn 只会被执行一次。
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	// 1. 如果这个 key 已经在账本里了（说明有人已经去查了）
	if c, ok := g.m[key]; ok {
		g.mu.Unlock() // 赶紧释放锁，让别人也能来查账本
		c.wg.Wait()   // 我就坐在这儿死等，直到那个人把数据拿回来
		return c.val, c.err
	}

	// 2. 如果账本里没有这个 key（说明我是第一个来的）
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	// 3. 真正去调用传进来的函数（比如去查数据库），这个过程可能很慢
	c.val, c.err = fn()
	c.wg.Done()

	// 4. 清理账本
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}