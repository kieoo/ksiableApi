package internal

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"ksiableApi/internal/helper"
	"ksiableApi/internal/log"
	"ksiableApi/internal/model"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ExecMapRecode map[string]int64

func ExecInit() {
	ExecMapRecode = make(map[string]int64)
	go func() {
		tick := time.Tick(1 * time.Minute)
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
	A:
		for {
			select {
			case <-tick:
				now := time.Now().Unix()
				for execFlag, t := range ExecMapRecode {
					// 3min 后还在执行, 却没有人在读这个任务(/loadLog 负责喂狗), 取消这个任务
					if now-t >= 180 {
						delete(ExecMapRecode, execFlag)
						log.Logger().Infof("Delete exec proc %s", execFlag)
					} else {
						log.Logger().Infof("Running exec proc %s", execFlag)
					}
				}
			case <-exit:
				log.Logger().Info("ExecInit exit.. ")
				break A
			}
		}
		log.Logger().Info("exit..for ")
	}()
}

func Exec(c *gin.Context) {
	req := &model.ExecReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("check your chooses:%s", err)})
		return
	}
	KubeConfBytesBase64Cookie, _ := c.Cookie("kube_config_yaml_base64")
	if len(KubeConfBytesBase64Cookie) > 0 {
		req.KubeConfBytes, _ = base64.StdEncoding.DecodeString(KubeConfBytesBase64Cookie)
	}
	// 不为空
	if len(req.KubeConfBytesBase64) > 0 {
		req.KubeConfBytes, _ = base64.StdEncoding.DecodeString(req.KubeConfBytesBase64)
	}
	// set cookie
	c.SetCookie("kube_config_yaml_base64", base64.StdEncoding.EncodeToString(req.KubeConfBytes), 7200, "/", "", false, false)

	if len(req.KubeConfBytes) <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprint("check your KubeConf")})
		return
	}
	// 解析 bytes -> clientcmdapi.config
	conf, err := helper.BuildConfig(req.KubeConfBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("decode kubeconfig failed")})
		return
	}

	if len(req.Command) <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("check your command")})
		return
	}

	if (strings.Contains(req.Command, "kill") || strings.Contains(req.Command, "rm ")) &&
		!req.AcceptKill {
		if !req.AcceptKill {
			c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("Dangerous  Operation - kill, rm...")})
			return
		}
		log.Logger().Infof("Exec kill user:%s, command:%s", req.KubeConfBytes, req.Command)
	}

	t := time.Now().UnixNano()

	uniqFlag := strconv.Itoa(int(t)) + "-" + strconv.Itoa(rand.Intn(1000))

	tmpFilePath := ""
	if dir, err := os.Getwd(); err == nil {
		tmpFilePath = path.Join(dir, "logs", "tmp")
	}

	rand.Seed(time.Now().UnixNano())
	tmpDir := path.Join(tmpFilePath, uniqFlag)
	err = os.MkdirAll(tmpDir, os.ModePerm)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("create tmp dir err:%s", err)})
		return
	}
	ch := make(chan struct{}, len(req.ExecContainers))
	exe := &ExecInfo{
		conf:       *conf,
		containers: req.ExecContainers,
		command:    req.Command,
		tmpDir:     tmpDir,
		execKey:    uniqFlag,
		cancel:     ch,
	}
	// 执行任务放入队列
	ExecMapRecode[uniqFlag] = time.Now().Unix()
	// 异步
	go func() {
		// 当podName=all, 裂变owner
		exe.FissionCon()
		// 执行 并输出到文件夹, 等待时间1800s
		exe.ExecCommand(1800)
		// 执行完成, 删除队列信号
		delete(ExecMapRecode, uniqFlag)
	}()

	c.JSON(http.StatusOK, gin.H{"exec_log_dir": uniqFlag})
}

type ExecInfo struct {
	conf       clientcmdapi.Config
	containers []model.ContainerInfo
	command    string
	tmpDir     string
	execKey    string
	cancel     chan struct{}
}

func (e *ExecInfo) FissionCon() {
	newContainers := new([]model.ContainerInfo)
	for _, container := range e.containers {
		if strings.ToUpper(container.PodName) != "ALL" {
			continue
		}
		oneConf := e.conf.DeepCopy()
		client := Client{rawConf: oneConf, contextName: container.ContextName}

		if err := client.InitClient(); err != nil {
			log.Logger().Warnf("FissionCon clientset %s creat error:%s", container.ContextName, err)
			continue
		}
		podsInfo := &model.PodsInfo{Context: container.ContextName, Cluster: container.ClusterName}
		err := helper.GetTargetPodInfo(client.Clientset, podsInfo, container.Namespace, container.OwnerReferenceName)
		if err == nil && len(podsInfo.Pods) > 0 {
			for _, pods := range podsInfo.Pods {
				for _, pod := range pods.PodNames {
					newContainer := model.ContainerInfo{
						ContextName:        container.ContextName,
						ClusterName:        container.ClusterName,
						Namespace:          container.Namespace,
						OwnerReferenceName: container.OwnerReferenceName,
						PodName:            pod,
						ContainerName:      container.ContainerName,
					}
					*newContainers = append(*newContainers, newContainer)
				}
			}
		}
	}
	if len(*newContainers) > 0 {
		e.containers = append(e.containers, *newContainers...)
	}
}

// ExecCommand 执行并分别写入 tmpDir/Cluster_Namespace_PodName_Container中
func (e *ExecInfo) ExecCommand(timeout int) {

	// 启动cancel时钟, 如果任务不在 ExecMapRecode中, 发送cancel信号
	go e.Cancel()

	wg := sync.WaitGroup{}
	for _, container := range e.containers {
		if len(container.PodName) == 0 || strings.ToUpper(container.PodName) == "ALL" {
			continue
		}
		oneConf := e.conf.DeepCopy()
		client := Client{rawConf: oneConf, contextName: container.ContextName}

		if err := client.InitClient(); err != nil {
			log.Logger().Warnf("ExecCommand clientset %s creat error:%s", container.ContextName, err)
			continue
		}
		fileName := path.Join(e.tmpDir, container.ClusterName+"_"+
			container.Namespace+"_"+container.PodName+"_"+container.ContainerName)

		wg.Add(1)
		// 并发执行
		go func(c Client, ca model.ContainerInfo, f string) {
			defer wg.Done()
			writer, err := os.OpenFile(f, os.O_CREATE, os.ModeExclusive)
			if err != nil {
				log.Logger().Warnf("ExecCommand file creat error:%s", err)
			}
			title := "# <===FROM==== " +
				ca.ClusterName + "__" + ca.Namespace + "__" + ca.PodName + "__" + ca.ContainerName +
				" ========>\n"
			_, err = writer.WriteString(title)
			if err != nil {
				log.Logger().Warnf("ExecCommand write file:%s error:%s", f, err)
			}
			done := make(chan struct{}, 1)
			// 停止exec执行
			exCancel := make(chan int)
			go func() {
				err = helper.RunExec(c.RestConf, c.Clientset,
					ca.Namespace, ca.PodName,
					ca.ContainerName, e.execKey, e.command, writer, exCancel)
				done <- struct{}{}
			}()

			select {
			case <-done:
				log.Logger().Infof("pod:%s, Done:exec command: %s", ca.PodName, e.command)
			case <-time.After(time.Duration(timeout) * time.Second):
				exCancel <- 1
				log.Logger().Infof("pod:%s, Timeout:exec command: %s", ca.PodName, e.command)
			case <-e.cancel:
				exCancel <- 1
				log.Logger().Infof("pod:%s, Kill:exec command: %s", ca.PodName, e.command)
			}

			if err != nil {
				log.Logger().Warnf("ExecCommand %s RunExec error: %s", f, err)
			}
			err = writer.Close()
			if err != nil {
				log.Logger().Warnf("ExecCommand close file file:%s error:%s", f, err)
			}
			err = os.Rename(f, f+"_finished")
			if err != nil {
				log.Logger().Warnf("ExecCommand finish file:%s error:%s", f, err)
			}
		}(client, container, fileName)
	}
	wg.Wait()
	// 表示执行完成
	succeed := path.Join(e.tmpDir, "_success")

	s, _ := os.Create(succeed)
	s.Close()
}

func (e *ExecInfo) Cancel() {
	for {
		if _, ok := ExecMapRecode[e.execKey]; !ok {
			for i := 0; i < len(e.containers); i++ {
				e.cancel <- struct{}{}
			}
			return
		}
		// 5s 检查一次执行队列是否需要删除
		time.Sleep(3 * time.Second)
	}
}
