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

	loader *singleflight.Group

	// 🌟 新装备：这个群组里所有缓存的统一过期时间
	ttl time.Duration
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
	defer mu.Unlock()

	// 1. 实例化一个全新的 Group
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
		ttl:       5 * time.Minute,
	}

	// 2. 登记到全局地图里
	groups[name] = g

	// 3. 把造好的 Group 交出去
	return g
}

// func (g *Group) SetTTL(d time.Duration) {
// 	g.ttl = d
// }

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
		//命中
		return v.(ByteView), nil
	}

	// 3. 缓存没命中 (miss)，我们要开启“加载模式”
	// 我们写一个新方法叫 load(key)，专门负责把数据搞回来
	return g.load(key)
}
func (g *Group) load(key string) (value ByteView, err error) {
	// 注意：返回值是 interface{}，需要类型转换一下
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			// 2. 调用指南针的 PickPeer(key) 方法，算一算这东西归谁管
			//这个pickpeer里面有一致性hash
			if peer, ok := g.peers.PickPeer(key); ok {
				// 我们写一个新方法 getFromPeer(peer, key) 派人去拿数据
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}

				log.Println("[GeeCache] 跨站抓取失败，准备尝试本地读取:", err)
			}
		}
		// 4. 兜底：如果指南针算出归自己管，或者去别人家拿失败了
		//去本地数据库查
		//为什么这里不直接在外面查完数据库再反回来，数据库一个都是公共的呀
		//这里不确定是否会前往其他服务器，有可能是属于直接这台服务器但是过期了
		//就是上一个peers不一定会进去
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
	//其实这个Get是从其他服务器那里取调用了
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// 1. 调用回调函数
	//这里这个getter其实就是main函数里面写的从数据库里读数据
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	// 2. 将数据包装成 ByteView
	value := ByteView{b: cloneBytes(bytes)} // 记得做一次深拷贝，保护数据

	// 3. 存入缓存，下次就快了
	g.mainCache.add(key, value, g.ttl)

	return value, nil
}

// RegisterPeers 注册一个 PeerPicker，用来选择远程节点
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) setToPeer(peer PeerGetter, key string, value []byte) error {
	req := &geecachepb.SetRequest{
		Group: g.name,
		Key:   key,
		Value: value,
	}
	res := &geecachepb.SetResponse{}
	//这里跨服务器去修改
	err := peer.Set(req, res)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("remote set failed: %s", res.Message)
	}
	return nil
}
func (g *Group) setLocally(key string, value []byte) {
	v := ByteView{b: cloneBytes(value)}
	g.mainCache.add(key, v, g.ttl)
}

func (g *Group) Set(key string, value []byte) error {
	//外面保证value一定不为空，这里要保证key一定不为空
	if key == "" {
		return fmt.Errorf("key is required")
	}
	//这里peers基本上不可能为空，因为他在main函数里面建好了
	//这里就是个防御性编程，防止万一
	//如果他为空就是一个纯的本地缓存，就是在main函数那里peers不设置
	if g.peers != nil {
		//这里就是直接去环里面找，看是否属于其他服务器
		if peer, ok := g.peers.PickPeer(key); ok {
			err := g.setToPeer(peer, key, value)
			if err != nil {
				log.Printf("[GeeCache] 跨站写入失败: %v\n", err)
				return err
			}
			return nil
		}
	}
	g.setLocally(key, value)
	return nil
}

func (g *Group) deleteFromPeer(peer PeerGetter, key string) error {
	req := &geecachepb.DeleteRequest{
		Group: g.name,
		Key:   key,
	}
	res := &geecachepb.DeleteResponse{}

	err := peer.Delete(req, res)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("remote delete failed")
	}
	return nil
}
func (g *Group) Delete(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok {
			err := g.deleteFromPeer(peer, key)
			if err != nil {
				log.Printf("[GeeCache] 跨站删除失败: %v\n", err)
				return err
			}
			return nil
		}
	}
	g.mainCache.delete(key)
	return nil
}
