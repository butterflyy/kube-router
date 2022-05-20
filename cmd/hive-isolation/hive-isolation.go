package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudnativelabs/kube-router/pkg/cri"
	"github.com/cloudnativelabs/kube-router/pkg/utils"
	"github.com/docker/docker/client"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"strconv"

	// nolint:gosec // we want to unconditionally expose pprof here for advanced troubleshooting scenarios
	_ "net/http/pprof"

	"k8s.io/klog/v2"
)

func main() {
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)

		if err.Error() == "Args error" {
			PrintHelp()
		}

		os.Exit(1)
	}
	os.Exit(0)
}

func PrintHelp(){
	fmt.Println("用法:")
	fmt.Println("	hive-isolation [选项] <参数>...")
	fmt.Println("")
	fmt.Println("操作pod的iptables的工具。")
	fmt.Println("")
	fmt.Println("选项")
	fmt.Println("	show <podname>     显示pod的iptables")
	fmt.Println("	clear <podname>    清除pod的iptables ")
	fmt.Println("	node-clear         清除节点所有的pod的iptables")
	fmt.Println("	cluster-clear      清除集群所有pod的iptables，DamonSet中使用，命令行下面未使用！")
	fmt.Println("")
	fmt.Println("	help               查看帮助")
	fmt.Println("")
	fmt.Println("示例")
	fmt.Println("	./hive-isolation show busybox-ds-vw7zv")
	fmt.Println("	./hive-isolation clear busybox-ds-vw7zv")
	fmt.Println("	./hive-isolation node-clear")

}

func GetCluster(inCluster bool)(kubernetes.Interface, error){
	var clientconfig *rest.Config
	var err error

	if inCluster{
		clientconfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.New("unable to initialize inclusterconfig: " + err.Error())
		}
	} else {
		clientconfig, err = clientcmd.BuildConfigFromFlags("", "/etc/kubernetes/admin.conf")
		if err != nil {
			return nil, errors.New("Failed to build configuration from CLI: " + err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(clientconfig)
	if err != nil {
		return nil, errors.New("Failed to create Kubernetes client: " + err.Error())
	}

	return clientset, nil
}

// GetNodeObject returns the node API object for the node
func GetNodeObject(clientset kubernetes.Interface) (*apiv1.Node, error) {
	// assuming kube-router is running as pod, first check env NODE_NAME
	nodeName := os.Getenv("NODE_NAME")
	if nodeName != "" {
		node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err == nil {
			return node, nil
		}
	}

	// if env NODE_NAME is not set then check if node is register with hostname
	hostName, _ := os.Hostname()
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), hostName, metav1.GetOptions{})
	if err == nil {
		return node, nil
	}


	{
		list, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil{
			panic("failed")
		}

		for _, node := range list.Items{
			fmt.Println("node name = %v", node.Name)
		}
	}


	return nil, fmt.Errorf("failed to identify the node by NODE_NAME, hostname, failed = %v", err)
}

func GetPodPid(containerURL string) (int, error) {
	_, containerID, err := cri.EndpointParser(containerURL)
	if err != nil {
		return 0, err
	}

	if containerID == "" {
		return 0, fmt.Errorf("containerID is empty")
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return 0, fmt.Errorf("failed to get docker client due to %v", err)
	}

	defer utils.CloseCloserDisregardError(dockerClient)

	//get pod inspect
	containerSpec, err := dockerClient.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get docker container spec due to %v", err)
	}

	pid := containerSpec.State.Pid
	if pid == 0{
		return 0, fmt.Errorf("Pid is 0, containerID is %v", containerID)
	}

	return containerSpec.State.Pid, nil
}

type PodInfo struct{
	namespace string
	name string
	pid int
}

func GetPodInfo(clientset kubernetes.Interface)([]PodInfo, error)  {
	stopCh := make(chan struct{})

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	podIndexer := informerFactory.Core().V1().Pods().Informer().GetIndexer()
	podLister := listers.NewPodLister(podIndexer)

	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	pods, err := podLister.List(labels.Everything())
	if err != nil{
		return nil, err
	}

	var podInfos []PodInfo

	for _, pod :=range pods {
		//only handle current node
		node, err := GetNodeObject(clientset)
		if err != nil{
			return nil, err
		}
		nodeIP, err := utils.GetNodeIP(node)
		if err != nil {
			return nil, err
		}

		if nodeIP.String() != pod.Status.HostIP{
			continue
		}

		//do not handle host network
		if pod.Spec.HostNetwork{
			continue
		}

		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True"{
				ready = true
				break
			}
		}

		if !ready{
			continue
		}

		containerURL := pod.Status.ContainerStatuses[0].ContainerID
		pid, err := GetPodPid(containerURL)
		if err != nil{
			return nil, fmt.Errorf("get pod pid failed, pidip(%v) %v", pod.Status.PodIP, err)
		}

		podInfos = append(podInfos, PodInfo{namespace: pod.Namespace, name: pod.Name, pid: pid})
	}

	return podInfos, nil
}

func Main() error {
	klog.InitFlags(nil)

	if len(os.Args) < 2{
		return fmt.Errorf("Args error")
	}

	type Type int32
	const(
		Show Type = 1
		Clear Type = 2
		NodeClear Type = 3
		ClusterClear Type = 4
	)

	cmdType := Show

	strCmdType := os.Args[1]

	switch strCmdType {
	case "show":
		cmdType = Show
	case "clear":
		cmdType = Clear
	case "node-clear":
		cmdType = NodeClear
	case "cluster-clear":
		cmdType = ClusterClear
	default:
		PrintHelp()
		return nil
	}

	clientset, err := GetCluster(cmdType == ClusterClear)
	if err != nil{
		return fmt.Errorf("Get cluster failed, %s", err)
	}

	podInfos, err := GetPodInfo(clientset)
	if err != nil{
		return fmt.Errorf("Get cluster failed, %s", err)
	}

	//fmt.Println("podInfos : %+v", podInfos)

	if cmdType == Show || cmdType == Clear{
		if len(os.Args) != 3 && len(os.Args) != 4{
			return fmt.Errorf("Args error")
		}

		var pid = 0

		if len(os.Args) == 3 {
			name := os.Args[2]

			var find_count = 0

			for _, podInfo := range podInfos {
				if podInfo.name == name {
					find_count++
					pid = podInfo.pid
				}
			}

			if find_count > 1{
				return fmt.Errorf("Find same pod name in different namespace")
			}
		} else{
			namespace := os.Args[2]
			name := os.Args[3]

			for _, podInfo := range podInfos {
				if podInfo.name == name && podInfo.namespace == namespace {
					pid = podInfo.pid
					break
				}
			}
		}

		if pid == 0 {
			return fmt.Errorf("Do not find pod")
		}

		var out []byte
		var err error
		if cmdType == Show{
			cmd := exec.Command("nsenter", "-n", "-t", strconv.Itoa(pid), "iptables", "-nvL")
			out, err = cmd.CombinedOutput()
		}else {
			cmd := exec.Command("nsenter", "-n", "-t", strconv.Itoa(pid), "iptables", "-F")
			out, err = cmd.CombinedOutput()
		}

		if err != nil{
			return fmt.Errorf("Exec cmd failed, %s", err)
		}

		fmt.Print(string(out))
	} else if cmdType == NodeClear || cmdType == ClusterClear {
		for _, podInfo := range podInfos{
			cmd := exec.Command("nsenter", "-n", "-t", strconv.Itoa(podInfo.pid), "iptables", "-F")
			err := cmd.Run()
			if err != nil{
				return fmt.Errorf("Pod %s Exec cmd failed, %s",podInfo.name, err)
			}
		}
	}

	return nil
}
