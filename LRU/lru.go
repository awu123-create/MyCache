package LRU

import "container/list"

/*
LRU(最近最少使用)缓存淘汰算法：
维护一个队列，如果某条记录被访问了，就将其移动到队列的尾部
此时队首即为最近最少访问的数据，可淘汰该条数据
*/

type Cache struct {
	//允许使用的最大内存
	maxBytes int64
	//当前已使用的内存
	nBytes int64

	//双向链表（用来实现队列）
	ll *list.List

	//缓存，list.Element内部指向entry结构体
	cache map[string]*list.Element

	//某条记录被移除时的回调函数，即key被淘汰时执行用户的自定义逻辑，可以为nil,
	onEvicted func(key string, value Value)
}

type entry struct {
	//存储key是为了在淘汰元素时，可以快速找到对应map的key并删除
	key   string
	value Value
}

// Value接口允许任意类型作为缓存的值，只要实现了Len()方法
type Value interface {
	Len() int
}

// Cache构造函数
func New(maxBytes int64, onEvicted func(key string, value Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onEvicted: onEvicted,
	}
}

/*
查找功能
1.在map中找到对应的双向链表的节点
2.将节点移动到双向链表的尾部
*/
func (c *Cache) Get(key string) (value Value, ok bool) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToBack(element)
		kv := element.Value.(*entry)
		return kv.value, ok
	}
	return
}

// 删除功能：删除最近最少访问的节点（队首节点）
func (c *Cache) RemoveOldest() {
	element := c.ll.Front()
	if element != nil {
		c.ll.Remove(element)
		kv := element.Value.(*entry)
		delete(c.cache, kv.key)
		c.nBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.onEvicted != nil {
			c.onEvicted(kv.key, kv.value)
		}
	}
}

/*
新增/修改功能:
1.如果key存在，则将对应节点移到末尾，更新value值和占用的内存
2.如果key不存在，则创建新的节点，并添加到末尾，更新占用的内存
3.如果占用内存超过了最大内存限制，就要删除最近最少访问的节点，直到符合内存限制
*/
func (c *Cache) Add(key string, value Value) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToBack(element)
		kv := element.Value.(*entry)
		c.nBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		element = c.ll.PushBack(&entry{key, value})
		c.cache[key] = element
		c.nBytes += int64(len(key)) + int64(value.Len())
	}

	for c.maxBytes != 0 && c.maxBytes < c.nBytes {
		c.RemoveOldest()
	}
}

// 供单元测试时使用，返回添加数据的数目
func (c *Cache) Len() int {
	return c.ll.Len()
}
