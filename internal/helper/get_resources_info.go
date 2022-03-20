package helper

import (
	"context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"ksiableApi/internal/log"
	"ksiableApi/internal/model"
	"sync"
)

func GetPodsObjectInfo(clientset kubernetes.Clientset, podsInfo *model.PodsInfo) error {
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	excludeNamespace := []string{"kube-system", "kube-public", "kube-node-lease", "monitoring", "ingress"}
	wg := sync.WaitGroup{}
	lock := sync.Mutex{}
	for _, ns := range namespaces.Items {

		if contains(excludeNamespace, ns.GetName()) {
			continue
		}
		// 并发每个namespace
		wg.Add(1)
		go func(namespace v1.Namespace) {
			defer wg.Done()
			log.Logger().Debugf("Debug : pod info get , ns:%v start...", namespace.GetName())
			// 获取ns->podlist
			pods, err := clientset.CoreV1().Pods(namespace.GetName()).List(context.TODO(), metav1.ListOptions{})
			log.Logger().Debugf("Debug : pod info get , ns:%v finish...", namespace.GetName())
			if err != nil {
				return
			}
			for _, pod := range pods.Items {
				podInfo := model.Pod{Ns: pod.GetNamespace(), PodName: pod.GetName()}
				// 如果是deployment, stateful等把父级名字也记录下来
				if len(pod.OwnerReferences) > 0 {
					podInfo.OwnerReferenceName = pod.OwnerReferences[0].Name
				}
				for _, container := range pod.Spec.Containers {
					podInfo.Containers = append(podInfo.Containers, container.Name)
				}
				// 信息放入返回列表
				lock.Lock()
				podsInfo.Pods = append(podsInfo.Pods, podInfo)
				lock.Unlock()
			}
		}(ns)
	}
	wg.Wait()
	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
