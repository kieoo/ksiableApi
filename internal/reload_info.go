package internal

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"ksiableApi/internal/helper"
	"ksiableApi/internal/log"
	"ksiableApi/internal/model"
	"net/http"
	"sync"
)

func ReloadInfo(c *gin.Context) {
	req := &model.ReloadInfoReq{}
	if c.BindJSON(req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("check your post data %s")})
		return
	}
	// 不为空
	var podsInfo []model.PodsInfo
	if len(req.KubeConfBytes) > 0 {
		// 解析 bytes -> clientcmdapi.config
		conf, err := helper.BuildConfig(req.KubeConfBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("decode kubeconfig failed")})
			return
		}
		podsInfo = GetContextsPodsInfoConfbytes(*conf, "All")
	}
	c.JSON(http.StatusOK, podsInfo)
	return
}

func GetContextsPodsInfoConfbytes(conf clientcmdapi.Config, contextName string) []model.PodsInfo {
	var podsInfosList []model.PodsInfo
	var lock sync.Mutex
	wg := sync.WaitGroup{}
	for context := range conf.Clusters {
		if context == contextName || contextName == "All" {
			// 避免currentContext 被替换
			oneConf := conf.DeepCopy()
			oneConf.CurrentContext = context
			client := Client{rawConf: oneConf, contextName: context}
			if err := client.InitClient(); err != nil {
				log.Logger().Warnf("GetContextsPodsInfo clientset creat error:%v", err)
				continue
			}
			podsInfo := &model.PodsInfo{Context: context}
			wg.Add(1)
			go func(clientset kubernetes.Clientset, podsInfo *model.PodsInfo) {
				defer wg.Done()
				// 获取context中, 所有pod信息
				err := helper.GetPodsObjectInfo(clientset, podsInfo)
				if err != nil {
					log.Logger().Warnf("GetContextsPodsInfo get info error, context:%v, err:%v", context, err)
					return
				}
				// pod信息不为空, 放入
				if len(podsInfo.Pods) > 0 {
					lock.Lock()
					podsInfosList = append(podsInfosList, *podsInfo)
					lock.Unlock()
				}
			}(client.Clientset, podsInfo)
		}
	}
	wg.Wait()
	return podsInfosList
}
