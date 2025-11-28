package MyCache

// 缓存值的抽象与封装
// ByteView是一个只读的不可变的数据容器，专门用来存储缓存中的值
// 使用[]byte类型，缓存值可以无视具体类型，只关心原始字节数据
type ByteView struct {
	b []byte
}

func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

// 返回数据的拷贝，避免外部修改原数据
func cloneBytes(b []byte) []byte {
	res := make([]byte, len(b))
	copy(res, b)
	return res
}
