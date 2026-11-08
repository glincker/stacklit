cmake_minimum_required(VERSION 3.20)
project(demo VERSION 1.0)

include(CTest)
add_subdirectory(src)
find_package(Boost REQUIRED)

add_library(core STATIC core.cpp)
add_executable(app main.cpp)
add_custom_target(docs COMMAND doxygen)

function(setup_target name)
  message(STATUS ${name})
endfunction()

macro(my_macro arg)
endmacro()
