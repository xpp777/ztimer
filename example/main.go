package main

import (
	"fmt"
	"github.com/xpp777/ztimer"
	"go.uber.org/zap"
	"time"
)

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
