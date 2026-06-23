cmake_minimum_required(VERSION 3.12)
project(Prism LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Prism 核心共享库
add_library(prism_engine SHARED
    prism_store.cpp
)

# 演示程序
add_executable(prism_demo main.cpp)
target_link_libraries(prism_demo prism_engine)