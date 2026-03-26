package monycachefinal // 🌟 重点 1：确保这行和你 geecache.go 的第一行一模一样！

import (
	"context"
	"fmt"
	"mony-cache_final/geecachepb"
	"mony-cache_final/consistenthash"
	"google.golang.org/grpc"
	"sync"
)

// --- 1. 服务端：负责接收别人的请求 ---

type grpcServer struct {
	geecachepb.UnimplementedGroupCacheServer
	addr string
}

func (s *grpcServer) Get(ctx context.Context, in *geecachepb.GetRequest) (*geecachepb.GetResponse, error) {
	g := GetGroup(in.GetGroup())
	if g == nil {
		return nil, fmt.Errorf("no such group: %s", in.GetGroup())
	}
	view, err := g.Get(in.GetKey())
	if err != nil {
		return nil, err
	}
	return &geecachepb.GetResponse{Value: view.ByteSlice()}, nil
}

// --- 2. 客户端（发射器）：负责向别人要数据 ---
// 🌟 重点 2：就是这玩意！编译器说没找着它！
type grpcGetter struct {
	addr string 
}

func (g *grpcGetter) Get(in *geecachepb.GetRequest, out *geecachepb.GetResponse) error {
	conn, err := grpc.Dial(g.addr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	client := geecachepb.NewGroupCacheClient(conn)
	resp, err := client.Get(context.Background(), in)
	if err != nil {
		return fmt.Errorf("could not get from peer %s: %v", g.addr, err)
	}
	out.Value = resp.GetValue()
	return nil
}

// --- 3. 管理者：负责维护哈希地图和所有的客户端 ---

type GRPCPool struct {
	geecachepb.UnimplementedGroupCacheServer
	self     string
	mu          sync.Mutex
	peers       *consistenthash.Map
	httpGetters map[string]*grpcGetter // 🌟 这里用到了上面的 grpcGetter
}

func NewGRPCPool(self string) *GRPCPool {
	return &GRPCPool{
		self:        self,
		httpGetters: make(map[string]*grpcGetter),
	}
}

func (p *GRPCPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(50, nil)
	p.peers.Add(peers...)
	for _, peer := range peers {
		p.httpGetters[peer] = &grpcGetter{addr: peer}
	}
}

func (p *GRPCPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		return p.httpGetters[peer], true
	}
	return nil, false
}