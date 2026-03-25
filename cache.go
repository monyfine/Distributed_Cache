package monycachefinal

import (
	"mony-cache_final/lru"
	"sync"
)

// cache 结构体：给 lru.Cache 加上互斥锁，保证并发安全
type cache struct{
	mu sync.Mutex
	lru *lru.Cache
	cacheBytes int64
}
/*
写一个加锁的 add 方法（保安帮忙存货）
当外面有人想存数据时，必须通过保安。保安先锁门，存完再开门。
核心技巧：延迟初始化（Lazy Initialization）。我们不在一开始就创建 lru.Cache，而是等第一次存数据的时候再创建，这样能省内存。
*/
func (c *cache)add(key string, value lru.Value){
	c.mu.Lock()
	defer c.mu.Unlock()

	// 延迟初始化：如果底层的 lru 还没创建，就在这里创建它
	if c.lru == nil{
		c.lru = lru.New(c.cacheBytes,nil)// 假设你的 lru.New 接收最大内存和回调函数
	}

	// 调用底层真正的 lru 去存数据
	c.lru.Add(key, value) 
}
// 同样的道理，取货也要加锁。
func (c *cache)get(key string)(value lru.Value,ok bool){
	c.mu.Lock()
	defer c.mu.Unlock()
	// 如果 lru 压根就还没创建，说明肯定没数据
	if c.lru == nil{
		return nil,false
	}
	if v,ok := c.lru.Get(key);ok{
		return v,ok
	}
	return nil,false
}