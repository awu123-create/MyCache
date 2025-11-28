package MyCache

import (
	"bytes"
	"fmt"
	"log"
	"testing"
)

func TestGetter(t *testing.T) {
	getter := GetterFunc(func(key string) ([]byte, error) {
		return []byte(key), nil
	})

	expect := []byte("helloworld")
	if v, _ := getter.Get("helloweold"); !bytes.Equal(v, expect) {
		t.Fatalf("callback failed")
	}
}

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGet(t *testing.T) {
	// loadCounts用来统计某个键调用回调函数的次数
	loadCounts := make(map[string]int, len(db))
	g := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[MyCache] search key", key)
			if v, ok := db[key]; ok {
				if _, ok := loadCounts[key]; !ok {
					loadCounts[key] = 0
				}
				loadCounts[key] += 1
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	for k, v := range db {
		// 对db中所有key执行第一次Get操作
		// 测试点：回调函数是否被调用，结果是否正确写入缓存，返回内容是否与数据库一致
		if view, err := g.Get(k); err != nil || view.String() != v {
			t.Fatalf("failed to get value of %s", k)
		}

		// 对db中key执行第二次Get操作
		// 测试点：缓存命中机制是否正常，Getter是否不会重复执行
		if _, err := g.Get(k); err != nil || loadCounts[k] != 1 {
			t.Fatalf("cache %s miss", k)
		}
	}

	// 测试访问不存在的key
	// 测试点：对不存在的数据，应该返回error，不应写入缓存
	if view, err := g.Get("unknown"); err == nil {
		t.Fatalf("the value of unknow should be empty,but %s got", view)
	}
}
