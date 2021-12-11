#时间轮定时器

## 时间轮场景

1. 管理socket心跳
```
    一个网络服务程序时需要管理大量客户端连接的，
    其中每个客户端连接都需要管理它的 timeout 时间。
    通常连接的超时管理一般设置为30~60秒不等，并不需要太精确的时间控制。
    另外由于服务端管理着多达数万到数十万不等的连接数，
    因此我们没法为每个连接使用一个Timer，那样太消耗资源不现实。
    用时间轮的方式来管理和维护大量的timer调度，会解决上面的问题。
```
2. 延时任务
```
   每个任务都需要管理它的 timeout 时间,
   当任务数达到数万数十万不等就会产生大量的goroutine,太消耗资源不显示
   用时间轮的方式来管理和维护会解决上面的问题
```

## 使用示例
1. 引入包
```
go get github.com/xiaomingping/timer
```
2. 测试
```
var (
	ZTimer = ztimer.NewAutoExecTimerScheduler()
)

func main() {
	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	for i := 0; i < 2000; i++ {
		f := ztimer.NewDelayFunc(foo, []interface{}{i, i * 3})
		_, err := ZTimer.CreateTimerAfter(f, time.Second*time.Duration(i*3))
		if err != nil {
			zap.S().Error(err)
			break
		}
	}
	time.Sleep(time.Hour)
}

func foo(v ...interface{}) {
	fmt.Printf("I am No. %d * 3 function, delay %d ms\n", v[0].(int), v[1].(int))
}

```
