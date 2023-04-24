package internal

import (
	"errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Client struct {
	rawConfBtyes []byte
	rawConf      *clientcmdapi.Config
	RestConf     *rest.Config
	Clientset    kubernetes.Clientset
	contextName  string
}

func (cli *Client) InitClient() error {
	// 优先使用config bytes
	if len(cli.rawConfBtyes) > 0 {
		if conf, err := clientcmd.NewClientConfigFromBytes(cli.rawConfBtyes); err == nil {
			if rawConf, err := conf.RawConfig(); err == nil {
				cli.rawConf = &rawConf
			} else {
				return err
			}
		} else {
			return err
		}
	}
	if cli.rawConf == nil {
		return errors.New("raw conf is nil")
	}
	// raw config
	conf := clientcmd.NewNonInteractiveClientConfig(*cli.rawConf, cli.contextName, nil, nil)

	var err error
	// rest config
	cli.RestConf, err = conf.ClientConfig()

	if err != nil {
		return err
	}

	// client set
	if clientset, err := kubernetes.NewForConfig(cli.RestConf); err == nil {
		cli.Clientset = *clientset
	} else {
		return err
	}

	return nil
}
