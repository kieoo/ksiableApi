package model

type ExecLogInfo struct {
	LoadLog
	Success    bool   `json:"success"`
	LogContent string `json:"content"`
}
