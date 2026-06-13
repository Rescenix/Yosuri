cmake_minimum_required(VERSION 3.12)
project(PrismStore LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 编译为共享库
add_library(prism_store SHARED prism_store.cpp)

# 可选：编译测试程序
add_executable(test_prism main.cpp)
target_link_libraries(test_prism prism_store)