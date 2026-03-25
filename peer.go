package monycachefinal

type PeerGetter interface{
	//这里前一个括号是传进去的后一个是得到的格式
	Get(group string,key string)([]byte,error)
}
type PeerPicker interface{
	// PickPeer 根据传入的 key 选择出对应的远程节点 
	PickPeer(key string)(peer PeerGetter,ok bool)
}