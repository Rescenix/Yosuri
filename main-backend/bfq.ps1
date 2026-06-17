# build_for_qq.ps1 —— 杉汐手机版一键编译脚本（PowerShell 版）
$env:GOOS="linux"
$env:GOARCH="arm64"
$env:CGO_ENABLED="0"
go build -o shanxi-server ./cmd/server