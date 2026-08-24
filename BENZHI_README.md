# BENZHI_README

基于 Go 实现的射线底片判读放行台 Web 项目，一款后端服务，已完整实现面向工业无损检测团队的射线底片判读放行台，使用真实 SQLite 事务和摘要寻址载荷存储贯通检测建档、不可变底片修订、自动完整性检查、人工缺陷判读、返拍替代复验、证据冻结、质量签发与凭据完整性验证，并提供响应式原生浏览器工作台和真实监听自检。

## 项目说明
- 项目：benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7
- 项目用途：已完整实现面向工业无损检测团队的射线底片判读放行台，使用真实 SQLite 事务和摘要寻址载荷存储贯通检测建档、不可变底片修订、自动完整性检查、人工缺陷判读、返拍替代复验、证据冻结、质量签发与凭据完整性验证，并提供响应式原生浏览器工作台和真实监听自检。
- Go 工具链：`golang:1.23.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7-arm64 linux/arm64
docker run -it benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
