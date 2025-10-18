package types

import "time"

type Task struct {
	TaskHash     string
	TaskName     string
	Creator      string
	ConnectionId string
	Msg          string
	RuleFile     string
	TaskType     string
	CheckData    string

	Condition Condition
}

type Condition struct {
	IntervalType string
	Interval     int

	StartTime time.Time
}

func NewTask(taskHash string, taskName string, creator string, connectionId string, msg string, ruleFile string, taskType string, intervalType string, interval int, startTime time.Time, checkData string) *Task {

	return &Task{
		TaskHash:     taskHash,
		TaskName:     taskName,
		Creator:      creator,
		ConnectionId: connectionId,
		Msg:          msg,
		RuleFile:     ruleFile,
		TaskType:     taskType,
		CheckData:    checkData,

		Condition: Condition{
			intervalType,
			interval,
			startTime,
		},
	}
}
