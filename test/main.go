// package main

// import (
// 	"fmt"
// 	"sync"
// 	"time"
// )

// func main() {
// 	var wg sync.WaitGroup

// 	// 老板派了 3 个打工人去干活
// 	for i := 1; i <= 3; i++ {
// 		wg.Add(1) // 记名：多了一个任务,任务加一
// 		go func(workerID int) {
// 			defer wg.Done() // 划掉名字：干完活了，报告老板  任务减一
			
// 			fmt.Printf("打工人 %d 开始干活...\n", workerID)
// 			time.Sleep(2 * time.Second) // 模拟干活耗时
// 			fmt.Printf("打工人 %d 干完了！\n", workerID)
// 		}(i)
// 	}

// 	fmt.Println("老板：我在这等你们全都干完...")
// 	wg.Wait() // 老板阻塞在这里，等点名册清零go  任务归零的时候结束等待
// 	fmt.Println("老板：所有人都干完了，下班！")
// }

package main

import (
	"fmt"
	"time"
)

func main() {
	// 创建一个传送带(管道)，只能放字符串
	ch := make(chan string)

	// 开一个小弟专门负责【生产】（往传送带放东西）
	go func() {
		fmt.Println("生产者：开始做包子")
		time.Sleep(2 * time.Second) // 做包子花了两秒
		ch <- "肉包子"                // 丢进传送带
		fmt.Println("生产者：包子丢上去了")
	}()

	// 主程序负责【消费】（从传送带拿东西）
	fmt.Println("消费者：我在等传送带送包子过来...")
	
	// 代码会阻塞在这里，直到传送带有东西出来！
	food := <-ch 
	
	fmt.Println("消费者：我吃到了", food)
}
// Do not communicate by sharing memory; instead, share memory by communicating.
// （不要通过共享内存来通信，而要通过通信来共享内存。）