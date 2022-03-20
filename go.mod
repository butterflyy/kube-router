module github.com/cloudnativelabs/kube-router

require (
	github.com/aws/aws-sdk-go v1.42.27
	github.com/butterflyy/go-iptables v0.6.2
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.0.1
	github.com/docker/docker v17.12.0-ce-rc1.0.20180816081446-320063a2ad06+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/moby/ipvs v1.0.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/osrg/gobgp v0.0.0-20211201041502-6248c576b118
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74
	golang.org/x/net v0.0.0-20220105145211-5b0dc2dfae98
	google.golang.org/grpc v1.43.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/cri-api v0.22.5
	k8s.io/klog/v2 v2.40.1
)

require (
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
)

replace github.com/containerd/containerd => github.com/containerd/containerd v1.5.8 // CVE-2021-32760 & CVE-2021-41103

replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2 // CVE-2021-41190

go 1.16
