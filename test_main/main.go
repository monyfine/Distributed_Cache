package main

import (
	"flag"
	"fmt"
	"log"
	monycachefinal "mony-cache_final"
	"net/http"
)

// 1. 模拟一个非常慢的底层数据库
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}
// func main() {
// 	// 2. 创建一个名为 "scores" 的缓存组，允许使用 2<<10 (2048) 字节的内存
// 	monycachefinal.NewGroup("scores", 2<<10, monycachefinal.GetterFunc(
// 		// 这是一个匿名函数，也就是你传给缓存大堂经理的“进货联系卡”
// 		func(key string) ([]byte, error) {
// 			log.Println("[缓慢的数据库] 正在从源头查询数据:", key)
			
// 			// 去咱们的假数据库(map)里找
// 			if v, ok := db[key]; ok {
// 				return []byte(v), nil
// 			}
// 			return nil, fmt.Errorf("数据库里根本没有这个人: %s", key)
// 		}))
// 	// 3. 实例化咱们的前台接待员 HTTPPool
// 	addr := "localhost:9999"
// 	peers := monycachefinal.NewHTTPPool(addr)
	
// 	log.Println("GeeCache is running at", addr)
	
// 	// 4. 启动 HTTP 服务，把请求都交给咱们的 peers (HTTPPool) 来处理
// 	// 这行代码会一直阻塞在这里运行，直到你按 Ctrl+C 关闭程序
// 	err := http.ListenAndServe(addr, peers)
// 	if err != nil {
// 		log.Fatal("服务器启动失败:", err)
// 	}
// }

func main() {
	var port int
	var api bool
	// 1. 使用 flag 包解析命令行参数
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	// 2.创建大堂经理 Group
	g := createGroup() // 封装一个创建 group 的函数，逻辑跟单机版一样

	// 3. 启动缓存服务节点
	addr := addrMap[port]
	peers := monycachefinal.NewHTTPPool(addr)
	
	// --- 👇 这里是分布式核心配置 👇 ---
	// 4. 把全国名单下发
	peers.Set(addrMap[8001],addrMap[8002],addrMap[8003]) // 传入 addrMap 里的所有地址
	
	// 5. 让大堂经理学会查地图
	g.RegisterPeers(peers) 
	// --------------------------------

	log.Println("geecache is running at", addr)
	
	// 开启一个协程去跑缓存节点的服务端
	go http.ListenAndServe(addr[7:], peers)

	// 如果 api 开关打开，再开一个端口给外部用户访问
	if api {
		// 监听 9999 端口，给用户提供一个简单的查询接口
		startAPIServer("http://localhost:9999", g)
	}

	select {} // 阻塞住，不让主程序退出
}
func createGroup() *monycachefinal.Group {
	// 1. 调用你核心包里的 NewGroup 函数
	// 参数 1: 组名 "scores"
	// 参数 2: 内存限制 2<<10 (也就是 2KB)
	// 参数 3: 这里的写法最关键！要用 monycachefinal.GetterFunc 强转一个匿名函数
	
	return monycachefinal.NewGroup("scores", 2<<10, monycachefinal.GetterFunc(
		func(key string) ([]byte, error) {
			// A. 打印一行醒目的日志，证明我们真的查了“数据库”
			log.Println("[缓慢的数据库] 正在查询源头数据:", key)

			// B. 去上面的 db 字典里找这个 key
			if v, ok := db[key]; ok {
				// C. 找到了，强转成 []byte 返回，错误填 nil
				return []byte(v), nil
			}

			// D. 没找到，返回一个 fmt.Errorf 错误
			return nil, fmt.Errorf("%s 不在数据库中", key)
		}))
}

func startAPIServer(apiAddr string, g *monycachefinal.Group) {
	// 1. 注册一个路由处理函数
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// A. 从网址里拿 key (比如 /api?key=Tom)
			key := r.URL.Query().Get("key")
			
			// B. 调用大堂经理的 Get 方法
			view, err := g.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			
			// C. 把拿到的字节流写回浏览器
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())
		}))
	
	// 2. 启动这个专门给用户用的 API 服务
	log.Println("API 服务器正在运行:", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}