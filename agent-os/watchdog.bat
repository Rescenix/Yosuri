@echo off
chcp 65001 >nul
cd /d "C:\Pro2026\re0\agent-os"
echo [%date% %time%] 不死鸟看门狗启动
:loop
echo [%date% %time%] 前台启动...
rescene.exe frontdesk
echo [%date% %time%] 前台退出(码=%errorlevel%)，5秒后重启
timeout /t 5 /nobreak >nul
goto loop