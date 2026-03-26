package monycachefinal

import (
	"fmt"
	"log"
	"mony-cache_final/geecachepb"
	"mony-cache_final/singleflight"
	"sync"
	"time"
)

// 回调接口：如果缓存没有，就调用这个接口的 Get 方法去源头找
type Getter interface {
	Get(key string) ([]byte, error)
}

// -----------------------------------------------------
// 这是一个极其经典的 Go 语言技巧：接口型函数！
// 它的作用是：让用户可以直接传一个普通函数进来，而不需要专门去写个结构体来实现接口。
type GetterFunc func(key string) ([]byte, error)

// 给这个函数类型绑定一个 Get 方法，让它实现 Getter 接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// -----------------------------------------------------

// 它是用户直接交互的对象。每个 Group 应该有一个名字，和一个具体的缓存实现。
type Group struct {
	name      string // 组名，比如 "scores"
	getter    Getter // 👈 新增这一行：缓存未命中时获取源数据的回调(callback)
	mainCache cache  // 具体的缓存实现（封装了你写的 LRU）

	// --- 👇 新增这一行：分发路由的指南针 👇 ---
	peers PeerPicker

	// 🌟 新增装备：使用 singleflight 防击穿
	loader *singleflight.Group

	// 🌟 新装备：这个群组里所有缓存的统一过期时间
	ttl       time.Duration 
}

/*
我们需要一个全局变量，把所有创建出来的 Group 都存起来，这样 http.go 才能通过名字找到它们。
逻辑：用一个 map 来存，Key 是组名，Value 是 Group 的指针。
注意：因为会有很多人同时查，所以还得配一把读写锁（sync.RWMutex）。
*/
var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

/*
实现 GetGroup 函数（档案查询员）
加上读锁（RLock），因为我们只是看一眼，不修改。
从全局 groups 地图里按名字找。
返回找到的那个 Group 指针。
*/
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

/*
实现 NewGroup 函数（档案登记处）
用户想创建一个新缓存组时调用它。
实例化一个 Group。
加写锁（Lock）。
把新组存进全局 groups 地图里。
返回这个新组。
*/
/*
全自动工厂：NewGroup
只需要传入组名，和这个组允许使用的最大内存。
工厂会在内部自动建好 Group，初始化保安室 (mainCache)，并登记到全局字典里。
如果用户没传回调函数，我们就报错（因为缓存查不到时系统就傻了）
*/
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	// 安检：如果没有提供回调函数，直接崩溃报错
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock() // 习惯性加上 defer，防止中间报错导致死锁

	// 1. 实例化一个全新的 Group
	g := &Group{
		name:      name,
		getter:    getter,                        // 👈 把传进来的 getter 组装进去
		mainCache: cache{cacheBytes: cacheBytes}, // 把刚刚说的“保安室”初始化一下
		loader:    &singleflight.Group{},         // 🌟 给新装备发弹药
		ttl:  5 * time.Minute, 					  // 🌟 默认值：5分钟后过期
	}

	// 2. 登记到全局地图里
	groups[name] = g

	// 3. 把造好的 Group 交出去
	return g
}

// 🌟 进阶亮点：提供一个修改 TTL 的方法（让用户可以自己定）
func (g *Group) SetTTL(d time.Duration) {
	g.ttl = d
}

// Get 方法：前台接待员(http.go)就是靠调用这个方法来拿数据的！
// 返回值有两个：我们刚写好的 ByteView 包装盒，以及 error 错误信息
func (g *Group) Get(key string) (ByteView, error) {
	// 1. 安检：如果客人传进来的 key 是空字符串 ""，直接返回错误
	if key == "" {
		// 返回一个空的包装盒，以及一段错误提示
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 2. 找保安拿数据：调用咱们结构体里的 mainCache (保安室) 的 get 方法
	// 接收返回值 v (数据) 和 ok (是否找到的布尔值)
	if v, ok := g.mainCache.get(key); ok {
		// 如果找到了 (ok 为 true)
		// 恭喜！缓存命中了！直接把拿到的 v 返回出去，错误填 nil
		return v.(ByteView), nil
	}

	// 3. 兜底：如果保安室没找到。
	// 正常的分布式缓存这里应该“触发回调函数去数据库查”，但咱们今天先从简，直接返回报错
	// return ByteView{}, fmt.Errorf("缓存没命中，找不到这个键: %s", key)

	// 2. 缓存没命中 (miss)，我们要开启“加载模式”
	// 我们写一个新方法叫 load(key)，专门负责把数据搞回来
	return g.load(key)
}
func (g *Group) load(key string) (value ByteView, err error) {
	// 🌟 魔法降临：把原本的逻辑用 loader.Do 包裹！
	// 注意：返回值是 interface{}，需要类型转换一下
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		// 1. 看看咱们有没有注册过“指南针”（peers）
		if g.peers != nil {
			// 2. 调用指南针的 PickPeer(key) 方法，算一算这东西归谁管
			if peer, ok := g.peers.PickPeer(key); ok {
				// 3. 【重点】既然 ok 为 true，说明算出是“别人”家管。
				// 我们写一个新方法 getFromPeer(peer, key) 派人去拿数据
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}

				log.Println("[GeeCache] 跨站抓取失败，准备尝试本地读取:", err)
			}
		}
		// 4. 兜底：如果指南针算出归自己管，或者去别人家拿失败了
		// 回归老样子：去本地数据库查
		return g.getLocally(key)
	})
	if err == nil {
		return viewi.(ByteView), nil // 从 interface{} 转回 ByteView
	}
	return
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	// 1. 组装 gRPC 认识的请求对象
	req := &geecachepb.GetRequest{
		Group: g.name,
		Key:   key,
	}
	// 2. 准备一个空的响应对象接数据
	res := &geecachepb.GetResponse{}

	// 🌟 核心：只传这两个 pb 对象，不再传 g.name 和 key 了！
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

/*
调用 g.getter.Get(key) 拿到原始字节 bytes。
万一数据库里也没有（err != nil），直接把错误甩出去。
如果拿到了数据，重点来了：我们要把这份新鲜的数据包装成 ByteView。
最关键的一步：调用一个叫 populateCache 的方法，把数据塞进缓存，这样下次就不用再查数据库了！
*/
func (g *Group) getLocally(key string) (ByteView, error) {
	// 1. 调用回调函数
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	// 2. 将数据包装成 ByteView
	value := ByteView{b: cloneBytes(bytes)} // 记得做一次深拷贝，保护数据

	// 3. 存入缓存，下次就快了
	g.populateCache(key, value)

	return value, nil
}

// 这个方法最简单，就是去调用保安室的 add。
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value, g.ttl)
}

// RegisterPeers 注册一个 PeerPicker，用来选择远程节点
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}
