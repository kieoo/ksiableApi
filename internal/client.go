package internal

import (
	"errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Client struct {
	RawConfBtyes []byte
	RawConf      *clientcmdapi.Config
	Conf         *clientcmd.ClientConfig
	contextName  string
}

func (cli *Client) InitClient() error {
	// 优先使用config bytes
	if len(cli.RawConfBtyes) > 0 {
		conf, err := clientcmd.NewClientConfigFromBytes(cli.RawConfBtyes)
		cli.Conf = &conf
		return err
	}
	if cli.RawConf != nil {
		conf := clientcmd.NewNonInteractiveClientConfig(*cli.RawConf, cli.contextName, nil, nil)
		cli.Conf = &conf
		return nil
	}
	return errors.New("RawConfBytes and RawConf is nil ")
}
