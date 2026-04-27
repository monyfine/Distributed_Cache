package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

/*
1. 为什么不用普通的 hash(key) % node_count？
如果你的机器数量变了（比如挂了一台，或者新加了两台），所有的 Key 对应的位置都会发生剧变！这会导致缓存瞬间在大规模范围内失效，直接把后端数据库冲垮（这叫缓存雪崩）。
2. 一致性哈希的绝招
哈希环：把 0 到 pow(2,32)-1 的数字首尾相连围成一个环。
虚拟节点：为了防止数据倾斜（某台机器存太多，某台太闲），我们给每台真实机器分身出成百上千个“幻影”。
*/
type Hash func(data []byte)uint32

type Map struct{
	hash Hash
	replicas int
	keys []int// 哈希环
	hashMap map[int]string // 虚拟节点 -> 真实节点
}

func New(replicas int, fn Hash) *Map {
	m :=&Map{
		hash: fn,
		replicas: replicas,
		hashMap: make(map[int]string),
	}
	// 2. 如果用户没传哈希函数，给个默认的 crc32
	if m.hash == nil {
		m.hash=crc32.ChecksumIEEE
	}
	return m
}

func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			// 3. 必须用 strconv.Itoa，把 i 变成 "0", "1", "2"
			hashValue := int(m.hash([]byte(strconv.Itoa(i)+key)))
			// 5. 将虚拟节点的哈希值加到环上
			m.keys = append(m.keys, hashValue)
			// 6. 映射虚拟节点到真实节点
			m.hashMap[hashValue]=key
		}
	}
	// 7. 环上的哈希值必须是有序的，二分查找的前提！
	sort.Ints(m.keys)
}

func (m *Map)Get(key string)string{
	// 1. 如果环上根本没有节点，直接返回
	if len(m.keys) == 0{return ""}
	// 2. 算 key 的哈希
	hashValue := m.hash([]byte(key))
	// 3. 在环上二分查找第一个 >= hash 的节点索引
	// 参数1：环的长度！
	idx := sort.Search(len(m.keys),func(i int) bool {
		return m.keys[i]>=int(hashValue)
	})
	// 4. 重点：先从 m.keys 拿到对应的哈希值，再从 m.hashMap 拿真实机器名
	// 注意 idx % len(m.keys) 是为了处理 idx == len(m.keys) 的情况（绕回环首）
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
