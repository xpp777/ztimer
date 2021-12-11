package ztimer

import (
	"go.uber.org/zap"
	"math"
	"sync"
	"time"
)

const (
	//MaxChanBuff 默认缓冲触发函数队列大小
	MaxChanBuff = 2048
	//MaxTimeDelay 默认最大误差时间
	MaxTimeDelay = 100
)

//TimerScheduler
/***********************************************************************************************************
对象: TimerScheduler
功能: 计时器调度器

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
type TimerScheduler struct {

	//当前调度器的最高级时间轮
	tw *TimeWheel

	//定时器编号累加器
	IDGen uint32

	//已经触发定时器的channel
	triggerChan chan *DelayFunc

	//互斥锁
	sync.RWMutex

	//所有注册的timerID集合
	IDs []uint32
}

//NewTimerScheduler
/***********************************************************************************************************
函数: NewTimerScheduler
功能: 创建定时器调度器   主要创建分层定时器，并做关联，并依次启动
参数: nil
返回: *TimerScheduler--定时器调度器

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func NewTimerScheduler() *TimerScheduler {

	//创建秒级时间轮
	secondTw := NewTimeWheel(SecondName, SecondInterval, SecondScales, TimersMaxCap)
	//创建分钟级时间轮
	minuteTw := NewTimeWheel(MinuteName, MinuteInterval, MinuteScales, TimersMaxCap)
	//创建小时级时间轮
	hourTw := NewTimeWheel(HourName, HourInterval, HourScales, TimersMaxCap)

	//将分层时间轮做关联
	hourTw.AddTimeWheel(minuteTw)
	minuteTw.AddTimeWheel(secondTw)

	//时间轮运行
	secondTw.Run()
	minuteTw.Run()
	hourTw.Run()

	return &TimerScheduler{
		tw:          hourTw,
		triggerChan: make(chan *DelayFunc, MaxChanBuff),
		IDs:         make([]uint32, 0),
	}
}

//CreateTimerAt
/***********************************************************************************************************
函数: CreateTimerAt
功能: 添加任务到分层时间轮中
参数: df--延迟调用函数对象,unixNano--时间戳
返回: uint32--任务ID,error--错误信息

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) CreateTimerAt(df *DelayFunc, unixNano int64) (uint32, error) {
	ts.Lock()
	defer ts.Unlock()

	ts.IDGen++
	ts.IDs = append(ts.IDs, ts.IDGen)
	return ts.IDGen, ts.tw.AddTimer(ts.IDGen, NewTimerAt(df, unixNano))
}

//CreateTimerAfter
/***********************************************************************************************************
函数: CreateTimerAfter
功能: 添加任务到分层时间轮中
参数: df--延迟调用函数对象,duration--间隔时间
返回: uint32--任务ID,error--错误信息

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) CreateTimerAfter(df *DelayFunc, duration time.Duration) (uint32, error) {
	ts.Lock()
	defer ts.Unlock()

	ts.IDGen++
	ts.IDs = append(ts.IDs, ts.IDGen)
	return ts.IDGen, ts.tw.AddTimer(ts.IDGen, NewTimerAfter(df, duration))
}

//CancelTimer
/***********************************************************************************************************
函数: CancelTimer
功能: 删除timer
参数: tID--任务ID
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) CancelTimer(tID uint32) {
	ts.Lock()
	ts.Unlock()
	//ts.tw.RemoveTimer(tID)  这个方法无效
	//删除timerID
	var index = -1
	for i := 0; i < len(ts.IDs); i++ {
		if ts.IDs[i] == tID {
			index = i
		}
	}

	if index > -1 {
		ts.IDs = append(ts.IDs[:index], ts.IDs[index+1:]...)
	}
}

//GetTriggerChan
/***********************************************************************************************************
函数: GetTriggerChan
功能: 获取计时结束的延迟执行函数通道
参数: nil
返回: *DelayFunc--延迟调用函数对象

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) GetTriggerChan() chan *DelayFunc {
	return ts.triggerChan
}

//HasTimer
/***********************************************************************************************************
函数: HasTimer
功能: 是否有时间轮
参数: tID--任务ID
返回: true--有,false--没有

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) HasTimer(tID uint32) bool {
	for i := 0; i < len(ts.IDs); i++ {
		if ts.IDs[i] == tID {
			return true
		}
	}
	return false
}

//Start
/***********************************************************************************************************
函数: Start
功能: 非阻塞的方式启动timerSchedule
参数: nil
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (ts *TimerScheduler) Start() {
	go func() {
		for {
			//当前时间
			now := UnixMilli()
			//获取最近MaxTimeDelay 毫秒的超时定时器集合
			timerList := ts.tw.GetTimerWithIn(MaxTimeDelay * time.Millisecond)
			for tID, timer := range timerList {
				if math.Abs(float64(now-timer.unixTs)) > MaxTimeDelay {
					//已经超时的定时器，报警
					zap.S().Error("want call at ", timer.unixTs, "; real call at", now, "; delay ", now-timer.unixTs)
				}
				if ts.HasTimer(tID) {
					//将超时触发函数写入管道
					ts.triggerChan <- timer.delayFunc
				}
			}
			time.Sleep(MaxTimeDelay / 2 * time.Millisecond)
		}
	}()
}

//NewAutoExecTimerScheduler
/***********************************************************************************************************
函数: NewAutoExecTimerScheduler
功能: 时间轮定时器 自动调度
参数: nil
返回: *TimerScheduler--时间轮定时器

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func NewAutoExecTimerScheduler() *TimerScheduler {
	//创建一个调度器
	autoExecScheduler := NewTimerScheduler()
	//启动调度器
	autoExecScheduler.Start()

	//永久从调度器中获取超时 触发的函数 并执行
	go func() {
		delayFuncChan := autoExecScheduler.GetTriggerChan()
		for df := range delayFuncChan {
			go df.Call()
		}
	}()

	return autoExecScheduler
}
