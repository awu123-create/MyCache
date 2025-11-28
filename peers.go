package MyCache

import pb "MyCache/MyCachePb"

type PeerPicker interface {
	// 根据传入的key选择相应节点PeerGetter
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter是用来跨节点获取数据的客户端接口
type PeerGetter interface {
	// 从对应的group中查找缓存值
	Get(in *pb.Request, out *pb.Response) error
}
