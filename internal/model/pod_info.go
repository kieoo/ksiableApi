package model

type PodsInfo struct {
	Context string `json:"context"`
	Pods    []Pod  `json:"pods"`
}

type Pod struct {
	Ns                 string   `json:"namespace"`
	OwnerReferenceName string   `json:"owner,omitempty"`
	PodName            string   `json:"pod_name"`
	Containers         []string `json:"containers"`
}

type PodsInfoAndConfig struct {
	KubeconfigYaml string     `json:"kube_config_yaml"`
	PodList        []PodsInfo `json:"plist"`
}
