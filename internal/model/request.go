package model

type ReloadInfoReq struct {
	KubeConfBytes []byte `json:"kubeConfBytes"`
}
