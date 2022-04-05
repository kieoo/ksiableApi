package model

type ContainerInfo struct {
	ContextName        string `json:"context"`
	ClusterName        string `json:"cluster"`
	Namespace          string `json:"ns"`
	OwnerReferenceName string `json:"owner,omitempty"`
	PodName            string `json:"pod_name"`
	ContainerName      string `json:"container_name"`
}
