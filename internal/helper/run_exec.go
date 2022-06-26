package helper

import (
	"fmt"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func RunExec(config *rest.Config, clientset kubernetes.Clientset, ns string,
	pod string, container string, key string, command string, out io.Writer, cancel chan int) error {

	cmd := []string{"sh", "-c", fmt.Sprintf("echo $$ > /tmp/%s; %s; rm /tmp/%s", key, command, key)}

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

	done := make(chan struct{}, 1)

	go func() {
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: out,
			Stderr: out,
		})
		done <- struct{}{}
	}()

	select {
	case <-done:
		return err
	case <-cancel:
		// 杀掉进程
		reqKill := clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod).
			Namespace(ns).
			SubResource("exec").
			Param("container", container)

		reqKill.VersionedParams(
			&v1.PodExecOptions{
				Container: container,
				Command:   []string{"sh", "-c", fmt.Sprintf("kill `cat /tmp/%s`; rm /tmp/%s", key, key)},
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec,
		)

		kill, err := remotecommand.NewSPDYExecutor(config, "POST", reqKill.URL())
		kill.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: out,
			Stderr: out,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
