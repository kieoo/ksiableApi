package internal

import (
	"encoding/base64"
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
	err := c.BindJSON(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("check your post data: %s", err)})
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

	var podsInfo []model.PodsInfo
	if len(req.KubeConfBytes) > 0 {
		// 解析 bytes -> clientcmdapi.config
		conf, err := helper.BuildConfig(req.KubeConfBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("decode kubeconfig failed")})
			return
		}
		podsInfo = GetContextsPodsInfoConfbytes(*conf, "", req.Namespace)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprint("check your KubeConf")})
		return
	}
	c.JSON(http.StatusOK, podsInfo)
	return
}

// GetContextsPodsInfoConfbytes 获取指定pod信息, contextName, namespace(空表示所有)
func GetContextsPodsInfoConfbytes(conf clientcmdapi.Config, contextName string, ns string) []model.PodsInfo {
	var podsInfosList []model.PodsInfo
	var lock sync.Mutex
	wg := sync.WaitGroup{}
	for name, context := range conf.Contexts {
		if name == contextName || contextName == "" {
			// 避免currentContext 被替换
			oneConf := conf.DeepCopy()
			oneConf.CurrentContext = name
			client := Client{rawConf: oneConf, contextName: name}
			if err := client.InitClient(); err != nil {
				log.Logger().Warnf("GetContextsPodsInfo clientset creat error:%s", err)
				continue
			}
			podsInfo := &model.PodsInfo{Context: name, Cluster: context.Cluster}
			wg.Add(1)
			go func(clientset kubernetes.Clientset, podsInfo *model.PodsInfo) {
				defer wg.Done()
				// 获取context中, 所有pod信息
				var err error
				if len(ns) > 0 {
					err = helper.GetTargetPodInfo(clientset, podsInfo, ns, "")
				} else {
					err = helper.GetPodsObjectInfo(clientset, podsInfo)
				}
				if err != nil {
					log.Logger().Warnf("GetContextsPodsInfo get info error, context:%s, err:%s", context, err)
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
