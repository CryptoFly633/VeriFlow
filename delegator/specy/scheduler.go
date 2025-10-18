package specy

import (
	"fmt"

	specyexecutor "github.com/cosmos/relayer/v2/specy/executor"
	specytypes "github.com/cosmos/relayer/v2/specy/types"

	"sync"
	"time"
)

var (
	everyBlockTasks            = make(map[string]*specytypes.Task)
	timeIntervalTaskGoroutines = make(map[string]chan struct{})
)

func CreateExecutor(iasReport string, enclavePK string) {
	if len(iasReport) == 0 && len(enclavePK) == 0 {
		// 命令行和配置文件都没有指定 调用engine接口查询 TODO
	}
	specyexecutor.CreateExecutorOnChain(iasReport, enclavePK)
}

func RegisterTask(task *specytypes.Task) {
	fmt.Printf("IntervalType %s \n", task.Condition.IntervalType)

	switch task.Condition.IntervalType {
	case "1":
		// 直接触发 task (goroutine)
		triggerTimeIntervalTask(task)

	case "0":
		// 将 task 注册到任务列表中 待爬区块的时候遍历触发
		fmt.Printf("将 task: %s 注册到任务列表中 待爬区块的时候遍历触发\n task: %+v \n", task.TaskHash, task)
		everyBlockTasks[task.TaskHash] = task
	default:
		fmt.Println("unsupportted task type")
	}
}

func triggerTimeIntervalTask(task *specytypes.Task) {
	stopCh := make(chan struct{})

	// 存储 goroutine 对象
	timeIntervalTaskGoroutines[task.TaskHash] = stopCh

	// 计算距离下一个时间节点的延迟时间
	delay := task.Condition.StartTime.Sub(time.Now())

	// 创建一个定时器，在延迟时间到达后开始触发定时任务
	timer := time.NewTimer(delay)

	go func() {
		// goroutine 中定时执行
		for ; true; <-timer.C {

			// 在定时任务中执行具体的操作
			fmt.Println("定时任务触发了！")

			specyexecutor.ExecuteTask(task)

			// 重新设置定时器，按照时间间隔触发下一次定时任务
			timer.Reset(time.Duration(task.Condition.Interval) * time.Second)

			select {
			case <-stopCh:
				fmt.Println(task.TaskHash, "stopped")
				return
			default:
				fmt.Println(task.TaskHash, "running")
			}
		}
	}()
}

func TriggerEveryBlockTasks(blockHeight int64) {
	if blockHeight%20 != 0 {
		return
	}

	// 将task放到数组中 方便后续操作
	tasks := make([]*specytypes.Task, 0, len(everyBlockTasks))
	for _, task := range everyBlockTasks {
		tasks = append(tasks, task)
	}

	// 每个子列表的大小
	batchSize := 10

	// 计算需要划分的子列表数量
	numBatches := (len(everyBlockTasks) + batchSize - 1) / batchSize

	// 分批处理列表
	for i := 0; i < numBatches; i++ {
		start := i * batchSize
		end := (i + 1) * batchSize

		if end > len(everyBlockTasks) {
			end = len(everyBlockTasks)
		}

		processTriggerEveryBlockTask(tasks[start:end])
	}
}

func processTriggerEveryBlockTask(tasks []*specytypes.Task) {
	if len(tasks) == 0 {
		return
	}
	if len(tasks) == 1 {
		fmt.Printf("Processing task: %+v \n", tasks[0])
		specyexecutor.ExecuteTask(tasks[0])
		return
	}

	var itemWg sync.WaitGroup
	itemWg.Add(len(tasks))
	for _, task := range tasks {
		go func(t *specytypes.Task) {
			defer itemWg.Done()

			fmt.Printf("Processing task: %+v \n", t)
			specyexecutor.ExecuteTask(t)

		}(task)
	}

	itemWg.Wait()
}

func UnregisterTask(taskHash string) {
	if everyBlockTasks[taskHash] != nil {
		removeEveryBlockTask(taskHash)
	} else {
		stopTimeIntervalTaskGoroutine(taskHash)
	}
}

func removeEveryBlockTask(hash string) {
	// 从 map 中移除 task
	delete(everyBlockTasks, hash)
}

func stopTimeIntervalTaskGoroutine(taskHash string) {
	stopCh, ok := timeIntervalTaskGoroutines[taskHash]
	if !ok {
		return
	}

	// 关闭 stop channel 以停止 goroutine
	close(stopCh)

	// 从 map 中移除 goroutine
	delete(timeIntervalTaskGoroutines, taskHash)
}
