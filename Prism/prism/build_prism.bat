@echo off
echo 🔥 正在编译 Prism 混沌核心...
cd /d C:\Pro2026\re0\Prism\prism
g++ -std=c++17 -O2 -o prism_demo.exe prism_store.cpp main.cpp
if %errorlevel% equ 0 (
    echo ✅ 编译成功！正在启动混沌核心...
    prism_demo.exe
) else (
    echo ❌ 编译失败，请检查代码。
    pause
)