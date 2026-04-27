package monycachefinal
/*
这里的 view，其实是我们需要定义的一个非常简单的结构体：ByteView（只读的数据视图）。
在分布式缓存里，为了防止别人拿到缓存数据后偷偷修改（导致内存里的数据被破坏），我们不直接给他们真实的切片，而是给一个只读的包装盒。
*/


type ByteView struct {
	b []byte 
}

func (v ByteView)Len()int{
	return len(v.b)
}

func (v ByteView)ByteSlice()[]byte{
	c := make([]byte,len(v.b))
	copy(c, v.b)
	return c
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}