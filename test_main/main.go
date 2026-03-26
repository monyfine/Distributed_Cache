package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"mony-cache_final"        // 🌟 换成你具体的包名
	"mony-cache_final/geecachepb"
	"google.golang.org/grpc"
)

// 1. 模拟一个底层的慢数据库 (MySQL)
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// 2. 启动一个 API 服务，方便我们用浏览器访问测试
// 比如访问 http://localhost:9999/api?key=Tom
func startAPIServer(apiAddr string, g *monycachefinal.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := g.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())
		}))
	log.Println("API 服务器运行在:", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func main() {
	var port int
	var api bool
	// 接收命令行参数：-port 指定 gRPC 端口，-api 指定是否开启 API 入口
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "是否作为 API 入口启动?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addr := fmt.Sprintf("localhost:%d", port)

	// 🌟 第一步：创建 Group (缓存主控室)
	// 定义如果缓存没命中，去哪里查 (这里去查模拟的 db)
	g := monycachefinal.NewGroup("scores", 2<<10, monycachefinal.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[数据回源] 正在从底层数据库查询 key:", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s 不存在", key)
		}))

	// 🌟 第二步：创建 GRPCPool (这里就是你问的创建地点！)
	// 它既是服务端(接收请求)，也是 PeerPicker(负责选节点)
	pool := monycachefinal.NewGRPCPool(addr)

	// 🌟 第三步：配置集群信息
	// 告诉池子，我们这个集群里有 3 个节点
	pool.Set("localhost:8001", "localhost:8002", "localhost:8003")

	// 🌟 第四步：建立联系 (这就是那个 PeerPicker 接口生效的瞬间！)
	g.RegisterPeers(pool)

	// 如果开启了 API 标志，就启动一个 HTTP 端口供我们手动查询
	if api {
		go startAPIServer(apiAddr, g)
	}

	// 🌟 第五步：启动 gRPC 服务端，监听对应的端口
	log.Println("gRPC 节点运行在:", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("监听失败:", err)
	}
	
	s := grpc.NewServer()
	// 注册服务：让别的节点能通过 gRPC 调到这个 pool
	geecachepb.RegisterGroupCacheServer(s, pool)

	if err := s.Serve(lis); err != nil {
		log.Fatal("启动失败:", err)
	}
}