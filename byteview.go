package monycachefinal
/*
这里的 view，其实是我们需要定义的一个非常简单的结构体：ByteView（只读的数据视图）。
在分布式缓存里，为了防止别人拿到缓存数据后偷偷修改（导致内存里的数据被破坏），我们不直接给他们真实的切片，而是给一个只读的包装盒。
*/

// 正确的结构体定义
type ByteView struct {
	b []byte // 字段 b 小写，表示私有，外部不能直接修改，起到保护作用
}

func (v ByteView)Len()int{
	return len(v.b)
}

func (v ByteView)ByteSlice()[]byte{
	// 1. 先用 make 造一个跟原来切片一样大的空盒子 (长度就是 len(v.b))
	c := make([]byte,len(v.b))

	// 2. 用 copy 函数，把 v.b 里面的数据，倒进刚刚造好的新盒子 c 里
	copy(c, v.b)

	return c
}

func cloneBytes(b []byte) []byte {
	// 1. 先造一个跟 b 一样大的空盒子 c
	// 使用 make([]byte, _________)
	c := make([]byte, len(b))

	// 2. 将 b 里面的内容倒进 c 里面
	// 使用 copy(_________, _________)
	copy(c, b)

	// 3. 返回这个崭新的盒子 c
	return c
}