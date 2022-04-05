package model

type PodsInfo struct {
	Context string `json:"context"`
	Cluster string `json:"cluster"`
	Pods    []Pod  `json:"pods"`
}

type Pod struct {
	Ns                 string   `json:"namespace"`
	OwnerReferenceName string   `json:"owner,omitempty"`
	PodNames           []string `json:"pod_name"`
	Containers         []string `json:"containers"`
}

type PodsInfoAndConfig struct {
	KubeconfigYaml       string     `json:"kube_config_yaml"`
	KubeconfigYamlBase64 string     `json:"kube_config_yaml_base64"`
	PodList              []PodsInfo `json:"plist"`
}
