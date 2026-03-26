//压力测试,中途会遇到一个vet的文件，系统查它的时候会很慢
// go test 在正式运行代码前，会自动调用一个叫 vet 的静态检查工具（检查代码里有没有明显的低级错误）。
// 由于你在 Windows 下，且 Go 安装在 D:\go，vet.exe 在扫描你的代码和引用的库（比如 unicode/bidi）时，被 Windows Defender 或某个杀毒软件拦截并进行扫描了。 这种扫描极其耗时，导致你的终端看起来像“死机”了一样。

// 一般可用  go test -v -x -run TestMapRace .
// 我们在运行测试时加上 -x 参数，这会让 Go 打印出它在后台做的每一个动作（编译、链接、运行）。
package monycachefinal // 确保包名正确

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentWrite(t *testing.T) {
	fmt.Println("start testing")
	// 1. 初始化你的缓存 (如果你的初始化函数不叫这个，请改成你实际的代码)
	c := &cache{
		cacheBytes: 1024,
	}

	// 🌟 2. 召唤点名册 (WaitGroup)
	var wg sync.WaitGroup

	fmt.Println("老板：开始派发 1000 个并发写任务...")

	for i := 0; i < 1000; i++ {
		wg.Add(1) // 🌟 记名：多了一个小弟任务

		go func(n int) {
			defer wg.Done() // 🌟 划掉名字：无论如何，干完活记得报告

			// 模拟高并发写
			key := fmt.Sprintf("key_%d", n)
			c.add(key, ByteView{b: []byte("value")},5*time.Second)
		}(i)
	}

	// 🌟 3. 老板在这里死等，直到点名册清零
	wg.Wait()
	fmt.Println("老板：所有小弟都干完活了！测试结束！")
}