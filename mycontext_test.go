package myContext

import (
	"fmt"
	"testing"
	"time"
)

// 一些简单的测试

var worker = func(ctx Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Println("working...")
		time.Sleep(time.Second)
	}
}

func TestWithCancel(t *testing.T) {
	parent, cancel := WithCancel(Background())
	context, _ := WithCancel(parent)
	go worker(context)
	go worker(context)
	time.Sleep(time.Second * 5)
	cancel()
	fmt.Println("main goroutine done...")
	time.Sleep(time.Second)
	fmt.Println(context.Err())
}

func TestWithTimeOut(t *testing.T) {
	context, cancel := WithTimeOut(Background(), time.Second*3)
	//context, cancel := WithTimeOut(Background(), time.Second*2)
	go worker(context)
	//go worker(context)
	time.Sleep(time.Second * 2)
	fmt.Println("main goroutine done")
	cancel()
	time.Sleep(time.Second)
	fmt.Println(context.Err())
}

func TestWithValue(t *testing.T) {
	// Value的查询需要O(n)的时间
	ctx := WithValue(Background(), "key1", "val1")
	ctx2, _ := WithCancel(ctx)
	ctx3 := WithValue(ctx2, "key3", "val3")
	fmt.Println(ctx.Value("key1"))  // val1
	fmt.Println(ctx.Value("key2"))  // nil
	fmt.Println(ctx2.Value("key1")) // val1
	fmt.Println(ctx2.Value("key2")) // nil
	fmt.Println(ctx3.Value("key1")) // val1
	fmt.Println(ctx3.Value("key3")) // val3
	fmt.Println(ctx3.Value("kdkd")) // 不存在的key一直要往上走直到根节点
}
