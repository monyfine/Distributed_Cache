package singleflight

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleFlight(t *testing.T) {
	var g Group
	var dbCallCount int32 // 记录真实的数据库查询次数

	// 模拟一个很慢的数据库查询函数
	slowDBQuery := func() (interface{}, error) {
		atomic.AddInt32(&dbCallCount, 1) // 真实查询次数 +1
		fmt.Println("   ---> [真实查询] 小明去底层数据库捞数据了...")
		time.Sleep(2 * time.Second) // 模拟查库耗时 2 秒
		return "周杰伦的夜曲", nil
	}

	var wg sync.WaitGroup
	// 瞬间派出 10 个小弟同时去查“周杰伦”
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// fmt.Printf("协程 %d 带着需求来了\n", id)
			
			// 呼叫 Single-flight！
			g.Do("周杰伦", slowDBQuery)
			
			// fmt.Printf("协程 %d 拿到了结果: %v\n", id, val)
		}(i)
	}

		for i := 1; i <= 100000000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// fmt.Printf("协程 %d 带着需求来了\n", id)
			
			// 呼叫 Single-flight！
			g.Do("周杰伦", slowDBQuery)
			
			// fmt.Printf("协程 %d 拿到了结果: %v\n", id, val)
		}(i)
	}
	wg.Wait()
	fmt.Printf("\n🏆 测试结束！10 个并发请求，真实查库次数：%d 次\n", dbCallCount)
}