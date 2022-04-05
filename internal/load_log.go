package internal

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"ksiableApi/internal/helper"
	"ksiableApi/internal/log"
	"ksiableApi/internal/model"
	"net/http"
	"os"
	"path"
	"strings"
)

func LoadLog(c *gin.Context) {
	req := &model.LoadLog{}
	err := c.BindJSON(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("Load failure:%s", err)})
		return
	}
	logFilePath := req.LogDir
	if dir, err := os.Getwd(); err == nil {
		logFilePath = path.Join(dir, "logs", "tmp", req.LogDir)
	}
	var buf []byte
	var finishedFileList []string
	var readFileList []string
	var runningFileList []string
	var success bool
	var logInfo model.ExecLogInfo

	logfiles, err := ioutil.ReadDir(logFilePath)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("Load no logs:%s", err)})
		return
	}

	// 统计当前状态
	for _, logfile := range logfiles {
		// finish 表示执行完成
		if strings.HasSuffix(logfile.Name(), "_finished") {
			finishedFileList = append(finishedFileList, logfile.Name())
		} else if strings.HasSuffix(logfile.Name(), "_read") {
			readFileList = append(readFileList, logfile.Name())
		} else if strings.HasPrefix(logfile.Name(), "_success") {
			success = true
		} else {
			runningFileList = append(runningFileList, logfile.Name())
		}
	}

	// 拼接 finished, 但还没读取的内容,
	// todo 可能包会后很大, 需要优化
	for _, finishedLogName := range finishedFileList {
		finishedLogPath := path.Join(logFilePath, finishedLogName)
		b, err := os.ReadFile(finishedLogPath)
		if err != nil {
			log.Logger().Warnf("LoadLog finishedLog: %s, read failure:%s", finishedLogName, err)
			continue
		}
		buf = append(buf, b...)
		// 已读标记
		err = os.Rename(finishedLogPath, finishedLogPath+"_read")
		if err != nil {
			log.Logger().Warnf("LoadLog remove finishedLog: %s, failure: %s", finishedLogName, err)
			continue
		}
	}
	// 如果丢包-即client返回的已读/finished文件有与当前readFileList有gap , 重新把已读标记的文件改为 (finished, no read)
	for _, readLogName := range readFileList {
		if !helper.Contains(req.Read, readLogName) &&
			!helper.Contains(req.Finished, strings.Split(readLogName, "_read")[0]) {
			b, err := os.ReadFile(path.Join(logFilePath, readLogName))
			if err != nil {
				log.Logger().Warnf("LoadLog finishedLog: %s, read failure", readLogName)
				continue
			}
			buf = append(buf, b...)
			finishedFileList = append(finishedFileList, readLogName)
		}
	}

	// 组组装结果
	logInfo.LogContent = string(buf)
	logInfo.LogDir = req.LogDir
	logInfo.Finished = finishedFileList
	logInfo.Read = readFileList
	logInfo.Running = runningFileList
	logInfo.Success = success

	c.JSON(http.StatusOK, logInfo)

	return
}
