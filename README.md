### 自己实现一个context包

###博客文章链接
[知乎](https://zhuanlan.zhihu.com/p/93458760)
[csdn](https://blog.csdn.net/qq_37667364/article/details/103224577)


2019.11.22 23:16  
已经基本实现，但是目前存在几个问题  
1. 个人感觉还依然存在一些bug，不合理的地方  
2. 核心的问题就是并发情况下的处理，互斥锁的是否加对  

#### 感悟
``
想要写一份考虑周全，优化好，健壮的代码并不容易，有经验则更容易  
没经验估计得试错才发现一些隐藏的bug，需要全局的考虑才能知道哪些  
地方可能容易出错，所以明天准备整理下代码，修正一些不合理的地方  
我觉得这也是个人需要去提升一个方向，如何做到代码的健壮性，性能好  
而不是一味只追求实现就行。``

#### 找到的bug集合（个人实现过程）
1. 由于sync.Mutex导致的死锁问题，cancel中详细解释了原因  


#### 一些发现
1. 使用了大量的懒加载的技术，即不先初始化变量，用到的时候才初始化  
应该处于资源性能的考虑吧

#### 附：官方使用context建议
1. Do not store Contexts inside a struct type; instead, pass a Context explicitly to each function that needs it. The Context should be the first parameter, typically named ctx.  
2. Do not pass a nil Context, even if a function permits it. Pass context.TODO if you are unsure about which Context to use.  
3. Use context Values only for request-scoped data that transits processes and APIs, not for passing optional parameters to functions.  
4. The same Context may be passed to functions running in different goroutines; Contexts are safe for simultaneous use by multiple goroutines.  


#### 个人使用context建议
1. 不要嵌套过多的context，构建非常复杂的context树，这样你可能会疑惑到底该调用哪个  
cancel，除非你真的非常理解context的内部机制  

