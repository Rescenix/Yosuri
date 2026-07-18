package handler

// EnsureDSNodeServer 原用于自动拉起 DS 浏览器代理（crack/server.js），现该代理已随
// crack/ 目录封存废弃、主链路走 /api/code/workflow 不再依赖它，故直接返回 nil 不再拉起，
// 避免 crack 删除后启动被缺失的 node 进程阻塞。函数保留签名仅为不破坏潜在引用。
func EnsureDSNodeServer() error {
	return nil
}
