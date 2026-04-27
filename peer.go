package monycachefinal

import "mony-cache_final/geecachepb"


// PeerPicker 是根据 key 选择节点的接口
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 接口 (在 geecache.go 中)
type PeerGetter interface {
	// 🌟 必须接收 pb 指针，且只返回 error
	Get(in *geecachepb.GetRequest, out *geecachepb.GetResponse) error
	Set (in *geecachepb.SetRequest,out *geecachepb.SetResponse) error
	Delete(in *geecachepb.DeleteRequest,out *geecachepb.DeleteResponse) error
}