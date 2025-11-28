package MyCache

import (
	"MyCache/ConsistenHash"
	pb "MyCache/mycachepb"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
)

// http.go基于http提供被其他节点访问的能力
const (
	defaultBasePath = "/_mycache/"
	defaultReplicas = 50
)

// 管理分布式节点之间的通信，负责根据key找到正确的远程节点并发送HTTP请求获取缓存数据
type HTTPPool struct {
	// 节点自身地址，包括IP和端口
	self string
	// 节点间通讯地址的前缀
	basePath string
	// 用来根据具体的key选择节点（一致性哈希）
	peers ConsistenHash.Map
	// 映射远程节点与对应的httpGetter，因为httpGetter与远程节点的地址baseURL有关
	httpGetters map[string]*httpGetter
	// 保护peers和httpGetters在并发环境下的安全访问
	mu sync.RWMutex
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// httpGetter是用来向远端节点发起HTTP请求获取缓存数据的客户端，对应于PeerGetter接口
type httpGetter struct {
	baseURL string
}

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	// 构建请求URL
	u := fmt.Sprintf(
		"%v%v%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)

	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %v", res.Status)
	}

	// 从服务器把缓存的结果读出来
	bytes, err := io.ReadAll(res.Body)
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}

// 在编译期检查httpGetter是否实现了PeerGetter接口
var _ PeerGetter = (*httpGetter)(nil)

// 让当前节点知道集群中有哪些节点，哪些节点负责哪些key
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.peers = ConsistenHash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// HTTPPool实现PeerPicker接口，每次本地缓存未命中时，由HTTPPool决定哪个远程节点负责处理该key
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		http.Error(w, "unexpected path", http.StatusBadRequest)
		return
	}

	p.Log("%s %s", r.Method, r.URL.Path)
	// 约定访问路径为 /basePath/groupname/key
	parts := strings.Split(strings.Trim(r.URL.Path[len(p.basePath):], "/"), "/")
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupname := parts[0]
	key := parts[1]

	group := groups[groupname]
	if group == nil {
		http.Error(w, "no such group:"+groupname, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}
