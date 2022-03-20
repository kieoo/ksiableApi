package helper

import (
	"errors"
	"github.com/imdario/mergo"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// BuildConfig kubeconf files path list
func BuildConfig(kubeconfigBytes []byte) (*clientcmdapi.Config, error) {
	if len(kubeconfigBytes) > 0 {
		config, err := clientcmd.Load(kubeconfigBytes)
		if err != nil {
			return nil, err
		}

		//if config.APIVersion == "" {
		//	config.APIVersion = clientcmdlatest.Version
		//}
		//if config.Kind == "" {
		//	config.Kind = "Config"
		//}
		if config.AuthInfos == nil {
			config.AuthInfos = map[string]*clientcmdapi.AuthInfo{}
		}
		if config.Clusters == nil {
			config.Clusters = map[string]*clientcmdapi.Cluster{}
		}
		if config.Contexts == nil {
			config.Contexts = map[string]*clientcmdapi.Context{}
		}

		return config, nil
	}
	return nil, errors.New("kubeconfig is nil")
}

// MergeConf 将多个kube config内容合并为 clientapi.config
func MergeConf(config *clientcmdapi.Config, kubeConfigFilesContents [][]byte) error {
	var kubeconfigs []*clientcmdapi.Config
	for _, fileContent := range kubeConfigFilesContents {
		if len(fileContent) == 0 {
			continue
		}
		if config, err := BuildConfig(fileContent); err == nil {
			kubeconfigs = append(kubeconfigs, config)
		}
	}
	if len(kubeconfigs) == 0 {
		return errors.New("config not found")
	}

	// first merge all of our maps
	mapConfig := clientcmdapi.NewConfig()

	for _, kubeconfig := range kubeconfigs {
		mergo.Merge(mapConfig, kubeconfig, mergo.WithOverride)
	}

	// merge all of the struct values in the reverse order so that priority is given correctly
	// errors are not added to the list the second time
	nonMapConfig := clientcmdapi.NewConfig()
	for i := len(kubeconfigs) - 1; i >= 0; i-- {
		kubeconfig := kubeconfigs[i]
		mergo.Merge(nonMapConfig, kubeconfig, mergo.WithOverride)
	}

	// since values are overwritten, but maps values are not, we can merge the non-map config on top of the map config and
	// get the values we expect.
	mergo.Merge(config, mapConfig, mergo.WithOverride)
	mergo.Merge(config, nonMapConfig, mergo.WithOverride)

	// kubeconfigYaml, err := yaml.Marshal(config)
	return nil
}

func Serializes2Yaml(config clientcmdapi.Config) ([]byte, error) {
	return clientcmd.Write(config)
}
