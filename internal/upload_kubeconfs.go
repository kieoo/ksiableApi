package internal

import (
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"ksiableApi/internal/helper"
	"ksiableApi/internal/model"
	"net/http"
)

func UpKubeconfs(c *gin.Context) {
	form, _ := c.MultipartForm()
	statusCode := http.StatusBadRequest
	if form == nil {
		c.JSON(statusCode, gin.H{"msg": "Kubeconfigs files is need"})
		return
	}

	if _, ok := form.File["files"]; !ok {
		c.JSON(statusCode, gin.H{"msg": "Kubeconfigs key files is need"})
		return
	}
	// 获取上传的文件
	files := form.File["files"]
	var ns string
	// 指定 namespace
	if _, ok := form.Value["namespace"]; ok {
		ns = form.Value["namespace"][0]
	}

	var filesContentsList [][]byte

	// 循环读取kubeconfig
	for _, file := range files {
		src, err := file.Open()
		if err != nil {
			continue
		}
		// 读取配置
		content, _ := ioutil.ReadAll(src)
		filesContentsList = append(filesContentsList, content)
		src.Close()
	}
	config := clientcmdapi.NewConfig()
	if len(filesContentsList) != 0 {
		// 获取RawConf
		if err := helper.MergeConf(config, filesContentsList); err == nil {
			if kubeconfigYaml, err := helper.Serializes2Yaml(*config); err == nil {
				c.Set("kconfig", kubeconfigYaml)
				statusCode = http.StatusOK
				// 获取所有context下所有pod信息
				podsInfoList := GetContextsPodsInfoConfbytes(*config, "", ns)
				kubeconfigBase64 := base64.StdEncoding.EncodeToString(kubeconfigYaml)
				resp := model.PodsInfoAndConfig{
					KubeconfigYaml:       string(kubeconfigYaml),
					KubeconfigYamlBase64: kubeconfigBase64,
					PodList:              podsInfoList,
				}
				c.SetCookie("kube_config_yaml_base64", kubeconfigBase64, 7200, "/", "192.168.1.3", false, false)
				c.JSON(statusCode, resp)
				return
			} else {
				c.JSON(statusCode, gin.H{"msg": err})
				return
			}
		} else {
			c.JSON(statusCode, gin.H{"msg": err})
			return
		}
	}
	c.JSON(statusCode, gin.H{"msg": "Decode kubeconfigs err"})
}
