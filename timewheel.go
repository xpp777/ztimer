package ztimer

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"sync"
	"time"
)



//TimeWheel
/***********************************************************************************************************
对象: TimeWheel
功能: 时间轮

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
type TimeWheel struct {
	//TimeWheel的名称
	name string

	//刻度的时间间隔，单位ms
	interval int64

	//每个时间轮上的刻度数
	scales int

	//当前时间指针的指向
	curIndex int

	//每个刻度所存放的timer定时器的最大容量
	maxCap int

	//当前时间轮上的所有timer
	timerQueue map[int]map[uint32]*Timer //map[int] VALUE  其中int表示当前时间轮的刻度,

	//下一层时间轮
	nextTimeWheel *TimeWheel

	//互斥锁（继承RWMutex的 RWLock,UnLock 等方法）
	sync.RWMutex
}

//NewTimeWheel
/***********************************************************************************************************
函数: NewTimeWheel
功能: 创建一个时间轮
参数: name--时间轮的名称,interval--每个刻度之间的duration时间间隔,scales--当前时间轮的轮盘一共多少个刻度(如我们正常的时钟就是12个刻度),maxCap--每个刻度所最大保存的Timer定时器个数
返回: *TimeWheel--时间轮对象

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func NewTimeWheel(name string, interval int64, scales int, maxCap int) *TimeWheel {
	tw := &TimeWheel{
		name:       name,
		interval:   interval,
		scales:     scales,
		maxCap:     maxCap,
		timerQueue: make(map[int]map[uint32]*Timer, scales),
	}
	//初始化map
	for i := 0; i < scales; i++ {
		tw.timerQueue[i] = make(map[uint32]*Timer, maxCap)
	}
	zap.S().Info("Init timerWhell name = ", tw.name, " is Done!")
	return tw
}

/*
	将一个timer定时器加入到分层时间轮中
	tID: 每个定时器timer的唯一标识
	t: 当前被加入时间轮的定时器
	forceNext: 是否强制的将定时器添加到下一层时间轮

	我们采用的算法是：
	如果当前timer的超时时间间隔 大于一个刻度，那么进行hash计算 找到对应的刻度上添加
	如果当前的timer的超时时间间隔 小于一个刻度 :
					如果没有下一轮时间轮
*/
//addTimer
/***********************************************************************************************************
函数: addTimer
功能: 将一个timer定时器加入到分层时间轮中
参数: tID--每个定时器timer的唯一标识,t--当前被加入时间轮的定时器,forceNext--是否强制的将定时器添加到下一层时间轮
返回: error--错误信息

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) addTimer(tID uint32, t *Timer, forceNext bool) error {
	defer func() error {
		if err := recover(); err != nil {
			errstr := fmt.Sprintf("addTimer function err : %s", err)
			zap.S().Error(errstr)
			return errors.New(errstr)
		}
		return nil
	}()

	//得到当前的超时时间间隔(ms)毫秒为单位
	delayInterval := t.unixTs - UnixMilli()
	//如果当前的超时时间 大于一个刻度的时间间隔
	if delayInterval >= tw.interval {
		//得到需要跨越几个刻度
		dn := delayInterval / tw.interval
		//在对应的刻度上的定时器Timer集合map加入当前定时器(由于是环形，所以要求余)
		tw.timerQueue[(tw.curIndex+int(dn))%tw.scales][tID] = t

		return nil
	}

	//如果当前的超时时间,小于一个刻度的时间间隔，并且当前时间轮没有下一层，经度最小的时间轮
	if delayInterval < tw.interval && tw.nextTimeWheel == nil {
		if forceNext == true {
			//如果设置为强制移至下一个刻度，那么将定时器移至下一个刻度
			//这种情况，主要是时间轮自动轮转的情况
			//因为这是底层时间轮，该定时器在转动的时候，如果没有被调度者取走的话，该定时器将不会再被发现
			//因为时间轮刻度已经过去，如果不强制把该定时器Timer移至下时刻，就永远不会被取走并触发调用
			//所以这里强制将timer移至下个刻度的集合中，等待调用者在下次轮转之前取走该定时器
			tw.timerQueue[(tw.curIndex+1)%tw.scales][tID] = t
		} else {
			//如果手动添加定时器，那么直接将timer添加到对应底层时间轮的当前刻度集合中
			tw.timerQueue[tw.curIndex][tID] = t
		}
		return nil
	}

	//如果当前的超时时间，小于一个刻度的时间间隔，并且有下一层时间轮
	if delayInterval < tw.interval {
		return tw.nextTimeWheel.AddTimer(tID, t)
	}
	return nil
}

//AddTimer
/***********************************************************************************************************
函数: AddTimer
功能: 添加一个timer到一个时间轮中(非时间轮自转情况)
参数: tID--每个定时器timer的唯一标识,t--当前被加入时间轮的定时器
返回: error--错误信息

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) AddTimer(tID uint32, t *Timer) error {
	tw.Lock()
	defer tw.Unlock()
	return tw.addTimer(tID, t, false)
}

//RemoveTimer
/***********************************************************************************************************
函数: RemoveTimer
功能: 删除一个定时器
参数: ID--任务ID
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) RemoveTimer(tID uint32) {
	tw.Lock()
	defer tw.Unlock()

	for i := 0; i < tw.scales; i++ {
		if _, ok := tw.timerQueue[i][tID]; ok {
			delete(tw.timerQueue[i], tID)
		}
	}
}

//AddTimeWheel
/***********************************************************************************************************
函数: AddTimeWheel
功能: 添加下层时间轮  比如给小时时间轮添加分钟时间轮，给分钟时间轮添加秒时间轮
参数: next--下层时间轮
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) AddTimeWheel(next *TimeWheel) {
	tw.nextTimeWheel = next
	zap.S().Info("Add timerWhell[", tw.name, "]'s next [", next.name, "] is succ!")
}

//run
/***********************************************************************************************************
函数: run
功能: 启动时间轮
参数: nil
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) run() {
	for {
		//时间轮每间隔interval一刻度时间，触发转动一次
		time.Sleep(time.Duration(tw.interval) * time.Millisecond)

		tw.Lock()
		//取出挂载在当前刻度的全部定时器
		curTimers := tw.timerQueue[tw.curIndex]
		//当前定时器要重新添加 所给当前刻度再重新开辟一个map Timer容器
		tw.timerQueue[tw.curIndex] = make(map[uint32]*Timer, tw.maxCap)
		for tID, timer := range curTimers {
			//这里属于时间轮自动转动，forceNext设置为true
			tw.addTimer(tID, timer, true)
		}

		//取出下一个刻度 挂载的全部定时器 进行重新添加 (为了安全起见,待考慮)
		nextTimers := tw.timerQueue[(tw.curIndex+1)%tw.scales]
		tw.timerQueue[(tw.curIndex+1)%tw.scales] = make(map[uint32]*Timer, tw.maxCap)
		for tID, timer := range nextTimers {
			tw.addTimer(tID, timer, true)
		}

		//当前刻度指针 走一格
		tw.curIndex = (tw.curIndex + 1) % tw.scales

		tw.Unlock()
	}
}

//Run
/***********************************************************************************************************
函数: Run
功能: 非阻塞的方式让时间轮转起来
参数: nil
返回: nil

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) Run() {
	go tw.run()
	zap.S().Info("timerwheel name = ", tw.name, " is running...")
}

//GetTimerWithIn
/***********************************************************************************************************
函数: GetTimerWithIn
功能: 获取定时器间隔内的Timer
参数: duration--时间对象
返回: map[uint32]*Timer 定时器对象map

编程: xiaomp
日期: 2021/12/11
***********************************************************************************************************/
func (tw *TimeWheel) GetTimerWithIn(duration time.Duration) map[uint32]*Timer {
	//最终触发定时器的一定是挂载最底层时间轮上的定时器
	//1 找到最底层时间轮
	leaftw := tw
	for leaftw.nextTimeWheel != nil {
		leaftw = leaftw.nextTimeWheel
	}

	leaftw.Lock()
	defer leaftw.Unlock()
	//返回的Timer集合
	timerList := make(map[uint32]*Timer)

	now := UnixMilli()

	//取出当前时间轮刻度内全部Timer
	for tID, timer := range leaftw.timerQueue[leaftw.curIndex] {
		if timer.unixTs-now < int64(duration/1e6) {
			//当前定时器已经超时
			timerList[tID] = timer
			//定时器已经超时被取走，从当前时间轮上 摘除该定时器
			delete(leaftw.timerQueue[leaftw.curIndex], tID)
		}
	}

	return timerList
}
