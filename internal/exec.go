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
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
	exe := &ExecInfo{*conf, req.ExecContainers, req.Command, tmpDir}
	// 异步
	go func() {
		// 当podName=all, 裂变owner
		exe.FissionCon()
		// 执行 并输出到文件夹, 等待时间1800s
		exe.ExecCommand(1800)
	}()

	c.JSON(http.StatusOK, gin.H{"exec_log_dir": uniqFlag})
}

type ExecInfo struct {
	conf       clientcmdapi.Config
	containers []model.ContainerInfo
	command    string
	tmpDir     string
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
			title := ca.ClusterName + "__" + ca.Namespace + "__" + ca.PodName + "__" + ca.ContainerName + "========>\n"
			_, err = writer.WriteString(title)
			if err != nil {
				log.Logger().Warnf("ExecCommand write file:%s error:%s", f, err)
			}
			done := make(chan struct{})
			go func() {
				err = helper.RunExec(c.RestConf, c.Clientset,
					ca.Namespace, ca.PodName,
					ca.ContainerName, e.command, writer)
				done <- struct{}{}
			}()

			select {
			case <-done:
				log.Logger().Debugf("pod:%s, exec command: %s done", ca.PodName, e.command)
			case <-time.After(time.Duration(timeout) * time.Second):
				log.Logger().Debugf("pod:%s, exec command: %s timeout", ca.PodName, e.command)
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