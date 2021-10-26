module kubetest

go 1.12

require (
	github.com/TaoYang526/kubetest v0.0.0-00010101000000-000000000000
	github.com/apache/incubator-yunikorn-core v0.11.0
	gonum.org/v1/netlib v0.0.0-20210927171344-7274ea1d1842 // indirect
	gonum.org/v1/plot v0.0.0-20190615073203-9aa86143727f
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.16.13
	k8s.io/apimachinery v0.16.13
	k8s.io/client-go v0.16.13
)

replace github.com/TaoYang526/kubetest => /home/yuteng/work/kubetest
