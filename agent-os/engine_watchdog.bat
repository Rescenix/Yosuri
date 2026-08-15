@echo off
chcp 65001 >nul
:: 聚合 API 引擎守护（不依赖桌面版 GUI，独立跑 8080）
:: 用计划任务开机自启 + 崩溃自动重启
cd /d "C:\Pro2026\re0\agent-os"
echo [%date% %time%] 聚合引擎看门狗启动
:loop
echo [%date% %time%] 启动聚合引擎（8080）...
rescene.exe --background
echo [%date% %time%] 引擎退出(码=%errorlevel%)，5秒后重启
timeout /t 5 /nobreak >nul
goto loop