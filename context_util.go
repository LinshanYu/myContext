package myContext

// 从父context查找最近的cancelCtx并从中删除child
func removeChild(parent Context, child canceler) {
	//这里的不需要加锁，树的增长方向向下
	//parentCancelCtx只会往树的根节点方向走
	//并且只读操作
	ctx, ok := parentCancelCtx(parent)
	if !ok {
		return
	}
	//由于go中的Mutex不可重入。之前的版本中会产生死锁问题
	//cancel中具体解释了原因
	ctx.lock.Lock()
	if ctx.children != nil {
		delete(ctx.children, child)
	}
	ctx.lock.Unlock()
}

// 从parent往树的根方向走找到最近的一个cancelCtx类型的Context
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	//使用类型断言
	for {
		switch c := parent.(type) {
		case *cancelCtx:
			return c, true
		case *valueCtx:
			//遇到valueCtx继续往上查找
			parent = c.Context
		case *timerCtx:
			return c.cancelCtx, true
		default:
			return nil, false
		}
	}
}

// 传播取消行为
// 大致功能：如果parent已经取消，那么将取消信号传递到parent这颗子树
// 否则将child加入到一个根节点方向最近的cancelCtx的children中
func propagateCancel(parent Context, child canceler) {
	if parent.Done() == nil {
		//父Context是不可取消的
		return
	}
	//parentCancelCtx往树的根节点方向找到最近的context是cancelCtx类型的
	if p, ok := parentCancelCtx(parent); ok {
		p.lock.Lock()
		if p.err != nil {
			//祖父cancelCtx已经取消，取消孩子节点
			child.cancel(false, p.err)
		} else {
			if p.children == nil {
				p.children = make(map[canceler]struct{})
			}
			//将child加入到祖父context中
			p.children[child] = struct{}{}
		}
		p.lock.Unlock()
	} else {
		//之前一直不太理解这里为什么出现else分支，找到了一篇知乎文章对这个问题有较好的解释
		//参考：https://zhuanlan.zhihu.com/p/68792989
		// 官方对使用context其中一条建议就是context不要放到结构体内部
		// 如果外部代码这样用了，结构体嵌入了context，类型不再是3大context（cancelCtx，timerCtx，valueCtx），
		// 但它实现了context的接口，可以调用接口的方法
		//导致的问题就是执行parentCancelCtx（在树往根方向查找最近的cancelCtx），因为类型原因是拿不到cancelCtx的
		//那么外部取消parent时，取消信号是无法往树的叶子方向传递，从而可能无法取消孩子context
		//所以这里启动一个goroutine来监控parent的取消信号，如果拿到了那么就可以顺利取消孩子context
		//两个case都是为了测试能够正常取消child
		//parent的cancel还是child的cancel哪个先调用都能退出select
		//如果发生parent的cancel不调用也是可能会出现goroutine泄露的，这里保证不了
		go func() {
			select {
			case <-parent.Done():
				child.cancel(false, parent.Err())
			case <-child.Done():
			}
		}()
	}
}
