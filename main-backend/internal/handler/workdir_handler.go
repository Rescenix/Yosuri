package handler

import (
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// PickWorkdir POST /api/workdir/pick —— 调起系统原生文件夹选择窗口。
// 浏览器无法安全取得本机绝对路径，因此不能用 HTML 文件输入控件代替它。
func PickWorkdir(c *gin.Context) {
	if runtime.GOOS != "windows" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "当前系统不支持原生文件夹选择器"})
		return
	}
	// FolderBrowserDialog 是 WinForms 的旧控件，只会打开树形选择框。这里改用
	// Windows Vista+ 的 IFileOpenDialog，并开启 FOS_PICKFOLDERS，得到资源管理器样式窗口。
	script := `
$ErrorActionPreference = 'Stop'
$source = @'
using System;
using System.Runtime.InteropServices;

namespace PrismNativeDialog {
  [ComImport, Guid("DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7")]
  public class FileOpenDialogRCW { }

  [ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("42F85136-DB7E-439C-85F1-E4075D135FC8")]
  public interface IFileDialog {
    [PreserveSig] int Show(IntPtr parent);
    void SetFileTypes(uint count, IntPtr specs); void SetFileTypeIndex(uint index); void GetFileTypeIndex(out uint index);
    void Advise(IntPtr events, out uint cookie); void Unadvise(uint cookie);
    void SetOptions(uint options); void GetOptions(out uint options);
    void SetDefaultFolder(IShellItem item); void SetFolder(IShellItem item); void GetFolder(out IShellItem item);
    void GetCurrentSelection(out IShellItem item); void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string name);
    void GetFileName([MarshalAs(UnmanagedType.LPWStr)] out string name); void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string title);
    void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string text); void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string text);
    void GetResult(out IShellItem item); void AddPlace(IShellItem item, int placement); void SetDefaultExtension([MarshalAs(UnmanagedType.LPWStr)] string extension);
    void Close(int hr); void SetClientGuid(ref Guid guid); void ClearClientData(); void SetFilter(IntPtr filter);
  }

  [ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("43826D1E-E718-42EE-BC55-A1E261C37BFE")]
  public interface IShellItem {
    void BindToHandler(IntPtr pbc, ref Guid bhid, ref Guid riid, out IntPtr ppv);
    void GetParent(out IShellItem parent);
    void GetDisplayName(uint sigdn, out IntPtr name);
    void GetAttributes(uint mask, out uint attributes);
    void Compare(IShellItem other, uint hint, out int order);
  }

  public static class FolderPicker {
    const uint FOS_PICKFOLDERS = 0x00000020;
    const uint FOS_FORCEFILESYSTEM = 0x00000040;
    const uint FOS_PATHMUSTEXIST = 0x00000800;
    const uint FOS_NOCHANGEDIR = 0x00000008;
    const uint SIGDN_FILESYSPATH = 0x80058000;
    public static string Pick() {
      IFileDialog dialog = (IFileDialog)new FileOpenDialogRCW();
      try {
        dialog.SetOptions(FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST | FOS_NOCHANGEDIR);
        dialog.SetTitle("Select Project Root");
        dialog.SetOkButtonLabel("选择文件夹");
        if (dialog.Show(IntPtr.Zero) != 0) return null;
        IShellItem item; dialog.GetResult(out item);
        IntPtr value; item.GetDisplayName(SIGDN_FILESYSPATH, out value);
        try { return Marshal.PtrToStringUni(value); } finally { Marshal.FreeCoTaskMem(value); }
      } finally { Marshal.ReleaseComObject(dialog); }
    }
  }
}
'@
Add-Type -TypeDefinition $source -ErrorAction Stop
$selected = [PrismNativeDialog.FolderPicker]::Pick()
if ($selected) { [Console]::Out.Write($selected) }
`
	pwsh := hiddenCommand("powershell.exe", "-NoProfile", "-STA", "-Command", script)
		out, err := pwsh.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开系统文件夹选择器"})
		return
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		c.JSON(http.StatusOK, gin.H{"cancelled": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "name": filepath.Base(path)})
}

// GetWorkdir GET /api/workdir —— 前端"添加工作目录"面板挂载时用这个同步真实值，
// 不能只信 localStorage：那只是 UI 展示层的缓存，agent 实际用的是后端 core.GetProjectRoot()。
func GetWorkdir(c *gin.Context) {
	root := core.GetProjectRoot()
	c.JSON(http.StatusOK, gin.H{
		"path": root,
		"name": filepath.Base(root),
	})
}

// SetWorkdir POST /api/workdir {"path": "main-frontend"} —— 真正切换 agent 的工作目录。
// path 支持相对路径（相对 GitRepoRoot，跟 /api/file-tree 返回的 path 字段对齐）和绝对路径。
// 切换后 read_file/write_file/edit_file/execute_command（含 rg 代码检索）/search_memory 全部立刻生效，
// 并落盘持久化到 ~/rescene_data/workdir.txt，下次启动自动恢复。
func SetWorkdir(c *gin.Context) {
	var body struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	target := body.Path
	if !filepath.IsAbs(target) {
		target = filepath.Join(GitRepoRoot, target)
	}
	target = filepath.Clean(target)

	if err := core.SetProjectRoot(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 工作目录切换后重建用户额外配置的外部 MCP 连接，使其 cwd/MCP_ROOT 跟随项目。
	ReinitMCP()

	// AgentFS：为每个项目会话开辟可追踪可回退的影子快照区（旁路，失败不影响主流程）
	OpenAgentFSSession(filepath.Base(target), target)

	c.JSON(http.StatusOK, gin.H{
		"path": target,
		"name": filepath.Base(target),
	})
}