package MyCache

import (
	pb "MyCache/MyCachePb"
	"MyCache/Singleflight"
	"fmt"
	"log"
	"sync"
)

// 负责与用户的交互，控制缓存存储和获取的主流程

// 缓存未命中时，系统需要向某个数据源加载数据，这个接口规定了加载的入口
type Getter interface {
	Get(key string) ([]byte, error)
}

// 让函数本身作为Getter接口的实现，使用户可以用一个简单函数定义缓存的回源加载逻辑，而不用写结构体
// 接口型函数：函数类型实现某一个接口
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group代表一个缓存的命名空间，每个Group拥有独立的缓存空间和数据加载逻辑
type Group struct {
	// 缓存命名空间的名称
	name string
	// 数据加载逻辑（缓存未命中时调用）
	getter Getter
	// 本地节点保存本机主数据缓存的LRU
	mainCache cache
	// 节点间数据共享
	peers PeerPicker
	// 使用singleFlight.Group来确保每个key只被加载一次
	loader *Singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup创建一个新的缓存命名空间
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	// 此处加锁是为了保证并发安全
	// 因为多个goroutine可能会同时创建Group或访问groups这个全局变量
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &Singleflight.Group{},
	}
	groups[name] = g
	return g
}

// GetGroup获取一个缓存命名空间
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 1.尝试从mainCache中获取缓存值
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[MyCache] hit")
		return v, nil
	}

	// 2.缓存未命中时，调用load方法加载数据
	return g.load(key)
}

// load方法使用PickPeer方法选择节点，如果不是本机节点，则调用getFromPeer方法从远程节点处获取
// 如果是本机节点或失败，则回退到getLocally方法调用用户回调函数g.Getter.Get()获取源数据
// 并且将源数据添加到mainCache中（使用populateCache方法）
func (g *Group) load(key string) (value ByteView, err error) {
	view, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			// 尝试从远程节点获取
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err := getFromPeer(peer, g.name, key); err == nil {
					return value, nil
				}
				log.Println("[MyCache] Failed to get from peer", err)
			}
		}
		// 从本地节点获取
		return g.getLocally(key)
	})

	if err == nil {
		return view.(ByteView), nil
	}
	return
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// 调用getter.Get()获取缓存值
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	value := ByteView{
		b: cloneBytes(bytes),
	}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// 将实现了PeerPicker接口的HTTPPool注入到Group中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func getFromPeer(peer PeerGetter, group string, key string) (ByteView, error) {
	req := &pb.Request{
		Group: group,
		Key:   key,
	}

	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}
