# Describe function in each pkg
1. cache
type are `Checkpoint` and `KubeDeployment`
`Checkpoint`
`KubeDeployment` afford `Build` function to create k8s deployment content, you can modify it via `withxxx` before building it.
2. cases
3. collector
4. common
There are some default arguments for chart,scheduler and envirnoment in `const.go`.
Some Validation and convertion are in `converter.go`  
5. kubeclient
6. monitor
7. painter
You can use `GetLinePoints` to get legel points and then assign it into to `Chart` type.
Get chart via `DrawChart` in the end.