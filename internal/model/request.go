package model

type ReloadInfoReq struct {
	KubeConfBytesBase64 string `json:"kube_config_yaml_base64" binding:"required"`
	KubeConfBytes       []byte `json:"kube_conf_bytes,omitempty"`
	Namespace           string `json:"namespace,omitempty"`
}

type ExecReq struct {
	KubeConfBytesBase64 string          `json:"kube_conf_bytes_base64" binding:"required"`
	KubeConfBytes       []byte          `json:"kube_conf_bytes,omitempty"`
	ExecContainers      []ContainerInfo `json:"containers" binding:"required"`
	Command             string          `json:"command" binding:"required"`
	AcceptKill          bool            `json:"accept_kill,omitempty"`
}

type LoadLog struct {
	LogDir   string   `json:"exec_log_dir" bidding:"required"`
	Running  []string `json:"running,omitempty"`
	Finished []string `json:"finished,omitempty" `
	Read     []string `json:"read,omitempty" `
}
