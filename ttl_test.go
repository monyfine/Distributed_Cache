package monycachefinal // 确保包名和你的 geecache/group.go 一致

import (
	"fmt"
	"testing"
	"time"
)

func TestTTL(t *testing.T) {
	fmt.Println("🌟 [测试开始] 验证缓存的 TTL 过期机制 🌟")

	// 1. 初始化一个带 Getter 的缓存群组
	db := map[string]string{
		"周杰伦": "夜曲",
	}
	getter := GetterFunc(
		func(key string) ([]byte, error) {
			fmt.Printf("   ---> 🚨 缓存没命中！正在从底层 DB 加载 key: %s\n", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s 不存在", key)
		},
	)
	
	g := NewGroup("scores", 2<<10, getter)
	
	// 🌟 2. 关键点：给这个群组设置极短的 TTL（比如 2 秒过期）
	// 注意：如果你刚才没在 Group 里加 SetTTL 方法，这里就直接改 g.ttl = 2 * time.Second
	g.ttl = 2 * time.Second

	// 3. 第一次请求，应该会穿透到 DB 拿数据
	fmt.Println("\n【第 1 次查询】 (期望：去 DB 拿)")
	v, _ := g.Get("周杰伦")
	fmt.Printf("   拿到结果: %s\n", v)

	// 4. 第二次请求，应该直接从缓存拿
	fmt.Println("\n【第 2 次查询】 (期望：命中缓存，不去 DB)")
	v, _ = g.Get("周杰伦")
	fmt.Printf("   拿到结果: %s\n", v)

	// 5. 睡 3 秒钟，熬过 2 秒的 TTL 死期！
	fmt.Println("\n😴 睡 3 秒钟... 让缓存子弹飞一会儿...")
	time.Sleep(3 * time.Second)

	// 6. 第三次请求，因为数据过期了，门卫会把它杀掉，然后重新去 DB 拿！
	fmt.Println("\n【第 3 次查询】 (期望：数据已过期，重新去 DB 拿)")
	v, _ = g.Get("周杰伦")
	fmt.Printf("   拿到结果: %s\n", v)
	
	fmt.Println("\n🏆 [测试结束] TTL 生存期管理通关！")
}