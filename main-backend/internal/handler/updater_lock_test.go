package handler

// 模拟 Windows 文件占用锁（Defender 实时扫描 / 旧实例未退出）下，
// renameWithRetry 与 acquireDownloadLock 的真实行为（2026-09-13）。

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// lockFileExclusive 以独占共享模式打开文件，模拟杀软实时扫描/其他进程占用。
// dwShareMode=0：拒绝一切共享（含删除/重命名），等价 Defender 扫描瞬间的锁。
// 返回句柄；调用方必须 CloseHandle 释放。
func lockFileExclusive(t *testing.T, path string) syscall.Handle {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	h, err := syscall.CreateFile(p,
		syscall.GENERIC_READ,
		0, // 独占：无 FILE_SHARE_READ/WRITE/DELETE
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		t.Fatalf("CreateFile 独占打开失败（模拟锁失败）: %v", err)
	}
	return h
}

// TestReplaceFileWithRetry_SurvivesTransientLock 核心场景：目标文件被另一进程独占，
// 500ms 后释放（模拟 Defender 扫描完成）。验证 replaceFileWithRetry（清理+rename
// 合并重试）在首次 Remove 就撞锁的情况下仍最终成功——旧实现 Remove 被锁会直接 return。
func TestReplaceFileWithRetry_SurvivesTransientLock(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "rescene-update-v0.3.9.zip.part")
	dst := filepath.Join(dir, "rescene-update-v0.3.9.zip")
	if err := os.WriteFile(src, []byte("mock zip bytes 1234567890"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 模拟：目标文件先被占用（比如上次残留 + 杀软正在扫它）
	if err := os.WriteFile(dst, []byte("old stale file"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := lockFileExclusive(t, dst)
	defer syscall.CloseHandle(h) // 保险：测试结束一定释放

	// 500ms 后释放锁（模拟 Defender 扫完就走了）
	releaseAt := time.Now().Add(500 * time.Millisecond)
	go func() {
		time.Sleep(time.Until(releaseAt))
		syscall.CloseHandle(h)
	}()

	start := time.Now()
	err := replaceFileWithRetry(src, dst)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("replaceFileWithRetry 应重试成功，实际失败: %v (耗时 %v)", err, elapsed)
	}
	if elapsed < 400*time.Millisecond {
		t.Fatalf("预期发生重试等待（耗时>=400ms），实际仅 %v", elapsed)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "mock zip bytes 1234567890" {
		t.Fatalf("rename 结果内容不对: data=%q err=%v", data, err)
	}
	t.Logf("✓ 瞬时锁下清理+rename 重试成功，耗时 %v", elapsed)
}

// TestReplaceFileWithRetry_NonShareErrorFastFail 非共享类错误（源不存在）必须立刻返回，
// 不做无谓重试。
func TestReplaceFileWithRetry_NonShareErrorFastFail(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	err := replaceFileWithRetry(filepath.Join(dir, "no-such-file.part"), filepath.Join(dir, "dst.zip"))
	if err == nil {
		t.Fatal("源文件不存在应报错")
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("非共享错误应快速失败，实际耗时 %v", elapsed)
	}
	t.Logf("✓ 非共享错误快速透传: %v", err)
}

// TestAcquireDownloadLock_SingleFlight 单飞锁：持锁期间再次获取必须失败，
// 释放后可再获取；崩溃残留（mtime 超 30 分钟）可强制接管。
func TestAcquireDownloadLock_SingleFlight(t *testing.T) {
	dir := t.TempDir()

	release1, err := acquireDownloadLock(dir)
	if err != nil {
		t.Fatalf("首次获取锁应成功: %v", err)
	}

	// 重复获取 → 必须报「已有下载在运行」
	if _, err := acquireDownloadLock(dir); err == nil {
		t.Fatal("持锁期间再次获取应失败")
	} else {
		t.Logf("✓ 持锁期间再次获取被挡: %v", err)
	}

	release1()
	// 释放后可再获取
	release2, err := acquireDownloadLock(dir)
	if err != nil {
		t.Fatalf("释放后应可再获取: %v", err)
	}
	release2()

	// 崩溃残留模拟：写一个锁文件，把 mtime 拨到 31 分钟前 → 应强制接管
	stale := filepath.Join(dir, ".download.lock")
	if err := os.WriteFile(stale, []byte("pid=99999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-31 * time.Minute)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatal(err)
	}
	release3, err := acquireDownloadLock(dir)
	if err != nil {
		t.Fatalf("崩溃残留锁应被强制接管: %v", err)
	}
	release3()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("接管后锁文件应已清除")
	}
	t.Logf("✓ 崩溃残留锁强制接管 OK")
}

// buildFakeUpdateZip 构造一个合法热补丁 zip：内含 >1MB、MZ 头、带版本串的假 rescene.exe。
// isLikelyWindowsExecutable 要求 >=1MB 且 MZ 魔数；exeContainsVersion 要求二进制内含版本串。
func buildFakeUpdateZip(t *testing.T, version string) []byte {
	t.Helper()
	exe := make([]byte, 1024*1024+64) // 超过 1MB 门槛
	copy(exe, "MZ")
	copy(exe[64:], []byte("rescene mock exe "+version))
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("rescene.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(exe); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestDownloadHotPatchZip_E2E 端到端 mock：httptest 提供 zip → downloadHotPatchZip
// 全流程（下载/版本校验/解压/提取/改名重试）→ 产物 rescene-new.exe 落盘且内容正确。
func TestDownloadHotPatchZip_E2E(t *testing.T) {
	const target = "0.3.99"
	zipBytes := buildFakeUpdateZip(t, target)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// 分批写，模拟真实网络流
		for i := 0; i < len(zipBytes); i += 32 * 1024 {
			end := i + 32*1024
			if end > len(zipBytes) {
				end = len(zipBytes)
			}
			if _, err := w.Write(zipBytes[i:end]); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, updateHotPatchFileName)

	// 模拟「上一次下载残留的旧包也被占用」：预先造一个旧 exe 目标 + 锁住它 400ms
	oldDest := []byte("MZold stale patch file")
	if err := os.WriteFile(dest, oldDest, 0o644); err != nil {
		t.Fatal(err)
	}
	h := lockFileExclusive(t, dest)
	releaseAt := time.Now().Add(400 * time.Millisecond)
	go func() {
		time.Sleep(time.Until(releaseAt))
		syscall.CloseHandle(h)
	}()

	err := downloadHotPatchZip(srv.URL+"/update.zip", dest, target)
	if err != nil {
		t.Fatalf("downloadHotPatchZip 端到端失败: %v", err)
	}

	// 产物校验：>1MB、MZ 头、内含版本串
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("产物未落盘: %v", err)
	}
	if fi.Size() < 1024*1024 {
		t.Fatalf("产物过小: %d", fi.Size())
	}
	data, err := os.ReadFile(dest)
	if err != nil || !bytes.HasPrefix(data, []byte("MZ")) {
		t.Fatalf("产物不是合法 exe: err=%v prefix=%q", err, data[:2])
	}
	if !bytes.Contains(data, []byte(target)) {
		t.Fatal("产物缺少目标版本串")
	}
	t.Logf("✓ 端到端热补丁下载成功：zip=%d bytes → %s (%d bytes, MZ+版本串✓)", len(zipBytes), dest, fi.Size())
}

// TestDownloadHotPatchZip_LockedStaleDest 关键回归：下载完成但旧目标被锁（杀软正在
// 扫旧包）→ replaceFileWithRetry 必须重试成功，绝不能像旧实现那样 Rename 直接失败。
func TestDownloadHotPatchZip_LockedStaleDest(t *testing.T) {
	const target = "0.3.100"
	zipBytes := buildFakeUpdateZip(t, target)

	// 锁目标 dest 更久（800ms），确保 replaceFileWithRetry 的第一个周期撞锁
	dir := t.TempDir()
	dest := filepath.Join(dir, updateHotPatchFileName)
	if err := os.WriteFile(dest, []byte("MZlocked old file"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := lockFileExclusive(t, dest)
	releaseAt := time.Now().Add(800 * time.Millisecond)
	go func() {
		time.Sleep(time.Until(releaseAt))
		syscall.CloseHandle(h)
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, bytes.NewReader(zipBytes))
	}))
	defer srv.Close()

	start := time.Now()
	err := downloadHotPatchZip(srv.URL+"/update.zip", dest, target)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("旧目标被锁时应重试成功，实际失败: %v", err)
	}
	if elapsed < 700*time.Millisecond {
		t.Fatalf("预期跨越锁窗口（>=700ms），实际 %v", elapsed)
	}
	fi, err := os.Stat(dest)
	if err != nil || fi.Size() < 1024*1024 {
		t.Fatalf("产物异常: size=%d err=%v", fi.Size(), err)
	}
	t.Logf("✓ 旧目标被锁 800ms 时仍重试成功，耗时 %v", elapsed)
}
