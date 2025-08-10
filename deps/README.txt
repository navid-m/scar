Boehm Garbage Collector Dependencies
=====================================

This directory contains the Boehm GC libraries and headers needed for the Scar compiler's 
garbage collection feature (-gc flag).

Required files to place in this directory:

For Windows (MinGW/GCC):
- libgc.a          (static library)
- libgc.dll.a      (import library for DLL, optional)

For Linux:
- libgc.a          (static library)
- libgc.so         (shared library, optional)

For macOS:
- libgc.a          (static library)
- libgc.dylib      (dynamic library, optional)

Header files (all platforms):
- gc.h             (main Boehm GC header)

Directory structure should be:
deps/
├── README.txt     (this file)
├── include/
│   └── gc.h
├── lib/
│   ├── windows/
│   │   ├── libgc.a
│   │   └── libgc.dll.a (optional)
│   ├── linux/
│   │   ├── libgc.a
│   │   └── libgc.so (optional)
│   └── darwin/
│       ├── libgc.a
│       └── libgc.dylib (optional)

How to obtain these files:
==========================

Windows (MSYS2):
1. Install MSYS2 and the gc package: pacman -S mingw-w64-x86_64-gc
2. Copy from: /mingw64/include/gc.h → deps/include/gc.h
3. Copy from: /mingw64/lib/libgc.a → deps/lib/windows/libgc.a
4. Copy from: /mingw64/lib/libgc.dll.a → deps/lib/windows/libgc.dll.a

Linux (Ubuntu/Debian):
1. Install: sudo apt-get install libgc-dev
2. Copy from: /usr/include/gc.h → deps/include/gc.h
3. Copy from: /usr/lib/x86_64-linux-gnu/libgc.a → deps/lib/linux/libgc.a

macOS (Homebrew):
1. Install: brew install bdw-gc
2. Copy from: /opt/homebrew/include/gc.h → deps/include/gc.h
3. Copy from: /opt/homebrew/lib/libgc.a → deps/lib/darwin/libgc.a
