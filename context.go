package myContext

import (
	"fmt"
	"sync"
	"time"
)

//实现一个自己的context

//Context 上下文管理接口
type Context interface {
	//返回一个-<chan struct{} 如果context实例是不可取消的，那么返回nil，比如空Context,valueCtx
	Done() <-chan struct{}
	//根据key拿到存储在context中的value
	//递归查找如果当前节点没有找到会往父节点
	Value(key interface{}) interface{}
	//返回任务取消的原因
	Err() error
	//返回任务执行的截止时间
	//非timerCtx类型则返回nil
	Deadline() (deadline time.Time, ok bool)
}

//canceler 取消context的接口
type canceler interface {
	cancel(removeFromParent bool, err error)
	Done() <-chan struct{}
}

//类型empty实现context接口作为一个空context类型
type empty int

//声明一个空Context实例
var bg empty = 0

//获取空类型context
func Background() Context {
	return &bg
}
func (b *empty) Done() <-chan struct{} {
	return nil
}
func (b *empty) Value(key interface{}) interface{} {
	return nil
}
func (b *empty) Err() error {
	return nil
}
func (b *empty) Deadline() (deadline time.Time, ok bool) {
	return
}

//固定变量，定义取消原因
var canceled = fmt.Errorf("%s", "Context canceled by calling the cancel method.")

//定义一个已经关闭的channel
var closedChan = make(chan struct{})

// 包初始化函数
// 关闭channel
func init() {
	close(closedChan)
}

//cancelCtx 实现了context和cancel接口
type cancelCtx struct {
	//保存parent context实例
	Context
	//用于锁下面用到的数据
	lock sync.Mutex
	//用于保存那些实现了canceler接口的子context
	//使用map是为了方便删除
	//方便取消孩子context
	children map[canceler]struct{}
	//实现取消需要用用到的channel
	done chan struct{}
	//取消原因
	err error
}

func (c *cancelCtx) cancel(removeFromParent bool, err error) {
	if err == nil {
		panic("internal error, Missing cancellation reason")
	}
	c.lock.Lock()
	//已经取消
	if c.err != nil {
		c.lock.Unlock()
		return
	}
	//取消
	c.err = err
	//1.关闭channel
	if c.done == nil {
		c.done = closedChan
	} else {
		close(c.done)
	}
	//2.执行孩子节点的cancel
	//孩子节点会持有父节点的lock
	for child := range c.children {
		//这里如果removeFromParent为true
		//会导致锁不可重入带来的死锁问题
		// 产生死锁的执行流程:
		// cancelCtx1.cancel
		// m1.Lock()
		//取消孩子context
		// cancelCtx2.cancel
		// m2.Lock()
		//取消孩子（假设无）
		//m2.Unlock()
		//removeChild
		//获取到父cancelCtx为cancelCtx1
		//使用m1锁
		// m1.Lock() 产生死锁，因为不可重入
		child.cancel(false, err)
	}
	//手动释放
	c.children = nil
	c.lock.Unlock()
	//从父节点中删除孩子节点
	if removeFromParent {
		//removeChild中使用了cancelCtx的锁
		removeChild(c.Context, c)
	}
}

func (c *cancelCtx) Done() <-chan struct{} {
	c.lock.Lock()
	if c.done == nil {
		c.done = make(chan struct{})
	}
	c.lock.Unlock()
	return c.done
}
func (c *cancelCtx) Err() error {
	//官方代码这里是加锁的
	//因为对err的操作即存在修改也存在读取
	//这里统一加互斥锁，如果不加不能保证读取到最新的数据
	//可以考虑使用读写锁，官方可能考虑到读写次数并不会相差太多所以没使用？
	c.lock.Lock()
	e := c.err
	c.lock.Unlock()
	return e
}

// cancelCtx构造实例
func newCancelCtx(parent Context) *cancelCtx {
	return &cancelCtx{
		Context: parent,
	}
}

// 在parent context基础上添加一个节点cancelCtx
func WithCancel(parent Context) (Context, func()) {
	ctx := newCancelCtx(parent)
	//传播取消行为
	propagateCancel(parent, ctx)
	return ctx, func() {
		ctx.cancel(true, canceled)
	}
}

// valueContext 用于存储key value
// 不可取消的context
type valueCtx struct {
	Context
	key   interface{}
	value interface{}
}

// 递归查找当前，查不到则往树的根节点方向继续查找
func (c *valueCtx) Value(key interface{}) interface{} {
	//if key == nil {
	//	return nil
	//}
	if c.key == key {
		return c.value
	}
	return c.Context.Value(key)
}

// 在parent Context节点下添加一个valueCtx
func WithValue(parent Context, key, value interface{}) *valueCtx {
	if key == nil {
		panic("nil key")
	}
	return &valueCtx{
		Context: parent,
		key:     key,
		value:   value,
	}
}

// 基于timer来实现超时取消的context
type timerCtx struct {
	*cancelCtx
	// 截止时间
	deadLine time.Time
	// 取消原因
	err error
	// 定时器
	*time.Timer
}

// 定义一个取消原因为超时的error
var timeOutErr = fmt.Errorf("%s", "timed out")

func (t *timerCtx) cancel(removeFromParent bool, err error) {
	//timer已经在初始化timerCtx中启动
	//这里的cancel作为timer定时执行的函数。这里直接调用cancel
	t.cancelCtx.cancel(false, err)
	//这里为什么这么写
	//因为在propagate中是将一个timerCtx加入到了parent（timerCtx继承cancelCtx所以实现了canceler接口可以被加入到cancelCtx的children）
	//调用上面的t.cancelCtx.cancel是移除不了的
	//timerCtx的parent应该是t.cancelCtx.Context
	if removeFromParent {
		removeChild(t.cancelCtx.Context, t)
	}
	t.cancelCtx.lock.Lock()
	//如果提前调用了cancel，下面的操作可以停止timer
	if t.Timer != nil {
		t.Timer.Stop()
		t.Timer = nil
	}
	t.cancelCtx.lock.Unlock()
}
func (t *timerCtx) Deadline() (deadline time.Time, ok bool) {
	return t.deadLine, true
}

// Deadline超时直接调用timeout即可
func WithDeadline(parent Context, deadline time.Time) (Context, func()) {
	//如果parent的deadline在当前context之前那么不需要开启新的定时器，直接由父context定时取消
	//当前context变为cancelCtx
	if d, ok := parent.Deadline(); ok && d.Before(deadline) {
		return WithCancel(parent)
	}
	ctx := &timerCtx{
		cancelCtx: newCancelCtx(parent),
		deadLine:  deadline,
	}
	propagateCancel(parent, ctx)
	dur := time.Until(deadline)
	if dur <= 0 { //立刻取消
		ctx.cancel(true, timeOutErr)
		return ctx, func() {
			ctx.cancel(false, canceled)
		}
	}
	//产生timer，AfterFunc会自己启动
	//这里为什么加锁？
	//前面的无论是propagate还是cancel内部都是有互斥处理
	//加锁是保证Timer必须被初始化才能使用，如果多个goroutine使用一个timerCtx，而这个timerCtx在另一个goroutine中
	//没初始化好，那么就会出问题
	ctx.cancelCtx.lock.Lock()
	defer ctx.cancelCtx.lock.Unlock()
	//确认这期间没有context被取消（比如parent被其他goroutine调用了cancel），不然无需初始化timer
	if ctx.err == nil {
		ctx.Timer = time.AfterFunc(dur, func() {
			ctx.cancel(true, timeOutErr)
		})
	}
	return ctx, func() {
		ctx.cancel(true, canceled)
	}
}

// 在parent Context下添加一个指定超时时间间隔的节点
// 基于WithDeadline，都是使用timerCtx
func WithTimeOut(parent Context, duration time.Duration) (Context, func()) {
	return WithDeadline(parent, time.Now().Add(duration))
}
