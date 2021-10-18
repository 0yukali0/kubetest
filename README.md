# kubetest
Testing tool for schedulers on K8s, support K8s default scheduler and YuniKorn scheduler.

## Key Metrics
* scheduled pods: pods that have started to run on kubelet(Decided by PodStatus#StartTime).

## Common Test Cases
* Throughput
   - Request 50,000 pods via different schedulers and then record the distributions of scheduled pods, draw results of different schedulers on the same chart.
* Node Fairness
   - Request a certain number of pods via different schedulers and then record the distributions of node usage, draw results on charts separately for different schedulers.

## Test cases for YuniKorn scheduler
* Queue Fairness
   - Prepare queues with different capacities, request pods with different number or resource for these queues, then record the usage of queues, draw results on the same chart.

## How to update package version and build this project
* Update
   - In `go.mod`, change the package version via `go mod edit -require` you need and use `go mode edit -replace github.com/TaoYang526/kubetest={the package you git clone}`.
   - Use `go mod tidy` to update.
* Build
   - Go to `pkg/cases/`,there are some main.go for different situations and use `go build` in one of them.
