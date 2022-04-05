package helper

import (
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func RunExec(config *rest.Config, clientset kubernetes.Clientset, ns string,
	pod string, container string, command string, out io.Writer) error {

	cmd := []string{"sh", "-c", command}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("exec").
		Param("container", container)

	// 非交互模式
	req.VersionedParams(
		&v1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: out,
		Stderr: out,
	})

	if err != nil {
		return err
	}

	return nil
}
