package monycachefinal

import "mony-cache_final/geecachepb"

// type PeerGetter interface{
// 	//这里前一个括号是传进去的后一个是得到的格式
// 	Get(group string,key string)([]byte,error)
// }
// type PeerPicker interface{
// 	// PickPeer 根据传入的 key 选择出对应的远程节点
// 	PickPeer(key string)(peer PeerGetter,ok bool)
// }
// PeerPicker 是根据 key 选择节点的接口
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 接口 (在 geecache.go 或 group.go 中)
type PeerGetter interface {
	// 🌟 必须接收 pb 指针，且只返回 error
	Get(in *geecachepb.GetRequest, out *geecachepb.GetResponse) error
}