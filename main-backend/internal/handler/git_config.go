package handler

import "backend/internal/ai/core"

// GitRepoRoot 仓库根——跟随项目根初始化（workdir 状态 > SHANXI_PROJECT_ROOT > 向上找 .git），
// 不再硬编码开发机路径。git 命令 cwd、文件树相对路径、workdir 相对解析都以它为基准；
// 切换工作目录后保持仓库根不变（git 会自动向上找 .git）。
var GitRepoRoot = core.GetProjectRoot()
