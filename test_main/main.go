package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"mony-cache_final"
	"mony-cache_final/geecachepb"

	"google.golang.org/grpc"
)

// 1. 模拟一个底层的慢数据库 (MySQL)
// 注意：我们的 Set/Delete 目前是操作分布式内存，并没有回写到这个 db 中。
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// 2. 启动一个 API 服务，这是用户与我们系统交互的入口
func startAPIServer(apiAddr string, g *monycachefinal.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// 从 URL 参数中获取 key
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "参数 key 不能为空", http.StatusBadRequest)
				return
			}

			switch r.Method {
			case http.MethodGet:
				// 读数据：GET /api?key=Tom
				view, err := g.Get(key)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(view.ByteSlice())

			case http.MethodPost, http.MethodPut:
				// 写数据：POST /api?key=Alice&value=100
				value := r.URL.Query().Get("value")
				if value == "" {
					http.Error(w, "写操作参数 value 不能为空", http.StatusBadRequest)
					return
				}
				err := g.Set(key,[]byte(value))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("成功写入 KV: [%s] = %s\n", key, value)))

			case http.MethodDelete:
				// 删数据：DELETE /api?key=Alice
				err := g.Delete(key)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("成功删除 Key: [%s]\n", key)))

			default:
				http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
			}
		}))

	log.Println("✅ API 对外交互网关运行在:", apiAddr)
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

	//第一步：创建 Group (缓存主控室)
	g := monycachefinal.NewGroup("scores", 2<<10, monycachefinal.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[数据回源] 正在从底层数据库查询 key:", key)
			if v, ok := db[key]; ok {
				return[]byte(v), nil
			}
			return nil, fmt.Errorf("%s 不存在", key)
		}))

	//第二步：创建 GRPCPool (节点通信池)
	pool := monycachefinal.NewGRPCPool(addr)

	//这里就是在配置一致性hash的那个环
	pool.SetPeers("localhost:8001", "localhost:8002", "localhost:8003")

	//把这个传给g中的peers，方便后续调用
	g.RegisterPeers(pool)

	// 🌟 第五步：如果开启了 API 标志，就启动 HTTP 交互网关
	if api {
		go startAPIServer(apiAddr, g)
	}

	// 🌟 第六步：启动 gRPC 服务端，监听对应的端口
	log.Printf("🚀 内部 RPC 节点运行在: %s", addr)
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