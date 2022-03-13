package internal

import (
	"github.com/gin-gonic/gin"
	"io/ioutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"ksiableApi/internal/helper"
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

	var filesContentsList [][]byte

	// 循环读取kubeconfig
	for _, file := range files {
		src, err := file.Open()
		if err != nil {
			continue
		}
		defer src.Close()
		// 读取配置
		content, _ := ioutil.ReadAll(src)
		filesContentsList = append(filesContentsList, content)
	}
	config := clientcmdapi.NewConfig()
	if len(filesContentsList) != 0 {
		// 获取RawConf
		if err := helper.MergeConf(config, filesContentsList); err == nil {
			if kubeconfigYaml, err := helper.Serializes2Yaml(*config); err == nil {
				statusCode = http.StatusOK
				c.JSON(statusCode, gin.H{"kubeconfigs": string(kubeconfigYaml)})
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
