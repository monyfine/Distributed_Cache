package lru

import (
	"container/list"
	"time"
)

// 1. 只要实现了 Len() int 的类型，都能作为缓存的值
type Value interface{
	Len() int
}
//这是双向链表里每一个节点（Node）真正存的东西
type entry struct{
	key   string
	value Value
	expireAT time.Time// 🌟 新增：过期时间（如果不设置就是默认零值，代表永久）
}

type Cache struct{
	maxBytes int64
	nBytes   int64
	ll       *list.List
	cache    map[string]*list.Element
	onEvicted func(key string,value Value)
}


// 4. New 函数（初始化）
//不理解onEvicted
func New(maxBytes int64,onEvicted func(string,Value))*Cache{
	return &Cache{
		maxBytes: maxBytes,
		ll: list.New(),
		cache:  make(map[string]*list.Element),
		onEvicted: onEvicted,
	}
}
// 🌟 新增的“狙击枪”：专门踢掉指定的某个节点
func (c *Cache) removeElement(ele *list.Element) {
	if ele != nil{
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache,kv.key)
		c.nBytes -= int64(len(kv.key)) + int64(kv.value.Len())// 扣减内存
		//不理解,不理解onEvicted这个东西
		if c.onEvicted != nil {
			c.onEvicted(kv.key,kv.value)// 通知人事总监
		}
	}
}
// 注意这里！把 c *Cache 写在括号里，表示这是 Cache 的方法！首字母 G 大写表示对外暴露！
func (c *Cache)Get(key string)(value Value,ok bool){
    // 1. 在 Go 里，查字典的黄金法则叫 "comma-ok 断言"
    // ele 就是 *list.Element
	if ele ,ok := c.cache[key]; ok{
        // 3. 难点来了：怎么把 Value 拿出来？
        // ele.Value 存的是任意类型 (any/interface{})，在我们的设计里，里面装的是 *entry 或 entry 结构体
		//所以需要 "类型断言" 把它还原回来！
		kv := ele.Value.(*entry)// 假设你链表里存的是 *entry 指针
		// 🌟 核心拦截机制：如果他有死期，并且现在的时间已经超过了死期
		//TTL
		if !kv.expireAT.IsZero()&&time.Now().After(kv.expireAT){
			// 哎呀，已经过期了！
			c.removeElement(ele) // 用狙击枪把他精准干掉
			return nil, false    // 告诉外面没查到
		}
        // 2. 既然找到了，把这个节点移到队头（代表最近使用）
        // 填空：调用 c.ll 的什么方法？参数传什么？
		c.ll.MoveToFront(ele)		
		return kv.value,true
	}
    // 5. 如果没找到，返回什么都不做（此时 value 和 ok 自动是它们类型的零值，也就是 nil 和 false）
	return nil,false
	//也可以直接写个return
}

// RemoveOldest 淘汰队尾最久未使用的元素（不需要传参数！）
func (c *Cache) RemoveOldest() {
	// 1. 去链表尾部抓那个最老的节点 (调用 c.ll 的 Back 方法)
	ele := c.ll.Back()
	
	c.removeElement(ele)// 直接开枪
}

func (c *Cache)Add(key string,value Value,ttl time.Duration){
	// 🌟 计算死期
	var expire time.Time
	if ttl > 0 {
		expire = time.Now().Add(ttl) // 当前时间 + 存活时间
	}
	if ele,ok := c.cache[key]; ok{
		// --- 场景 A：Key 已存在（更新） ---
		// 1. 把节点移到最前面（最近被使用）
		c.ll.MoveToFront(ele)
		// 2. 类型断言拿到 entry 指针
		kv := ele.Value.(*entry)//这个是旧的
		// 3. 更新内存计数：加上新值的长度，减去旧值的长度（注意：key 没变，不用算 key）
		c.nBytes += int64(value.Len())-int64(kv.value.Len())
		// c.cache[key]=c.ll.Front()
		//这个不用更新 这个 ele 指向的内存地址（指针值）从始至终都没有变！ 它还是原来那个节点对象，只是它在链表里的位置变了。
		kv.value=value
		kv.expireAT = expire
	}else{
		// ！！！注意这里：存入的是整个 entry 的指针 ！！！
		ele := c.ll.PushFront(&entry{key,value,expire})
		c.cache[key] = ele
		// 重点！重点！重点！
		// key 是字符串，用内置函数 len(key)
		// value 是接口，用方法 value.Len()
		c.nBytes += int64(len(key))+int64(value.Len())
	}

	// --- 场景 C：检查大坝水位（淘汰循环） ---
	// 只要设了最大内存限制，且当前内存爆了，就一直踢掉最老的数据
	for c.maxBytes != 0 && c.maxBytes < c.nBytes{
		c.RemoveOldest()
	}
}