package helper

import (
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"strings"
)

func RunExec(config *rest.Config, clientset kubernetes.Clientset, ns string,
	pod string, container string, command string, out io.Writer, cancel chan int) error {

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
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	done := make(chan struct{})

	go func() {
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  strings.NewReader(command),
			Stdout: out,
			Stderr: out,
		})
		done <- struct{}{}
	}()

	select {
	case <-done:
		return err
	case <-cancel:
		exec.Stream(remotecommand.StreamOptions{
			Stdin:  strings.NewReader("\\003"),
			Stdout: out,
			Stderr: out,
		})
	}

	return nil
}
