// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Toolchain management for scar compiler

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func tryPkgConfig(packageName string) []string {
	cmd := exec.Command("pkg-config", "--cflags", "--libs", packageName)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	flags := strings.Fields(strings.TrimSpace(string(output)))
	return flags
}

func findBundledBoehm() []string {
	execPath, err := os.Executable()
	if err != nil {
		if wd, err := os.Getwd(); err == nil {
			execPath = wd
		} else {
			return nil
		}
	}
	scarDir := filepath.Dir(execPath)
	if strings.Contains(execPath, "go-build") || strings.Contains(execPath, "Temp") {
		if wd, err := os.Getwd(); err == nil {
			scarDir = wd
		}
	}
	depsDir := filepath.Join(scarDir, "deps")
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		return nil
	}
	var libSubdir string
	switch runtime.GOOS {
	case "windows":
		libSubdir = "windows"
	case "linux":
		libSubdir = "linux"
	case "darwin":
		libSubdir = "darwin"
	default:
		return nil
	}
	var (
		includePath = filepath.Join(depsDir, "include")
		libPath     = filepath.Join(depsDir, "lib", libSubdir)
		gcHeader    = filepath.Join(includePath, "gc.h")
		gcLib       = filepath.Join(libPath, "libgc.a")
	)
	if _, err := os.Stat(gcHeader); err == nil {
		if _, err := os.Stat(gcLib); err == nil {
			return []string{
				"-I", includePath,
				"-L", libPath,
				"-lgc",
			}
		}
	}

	return nil
}

func findMinGWBoehm() []string {
	var searchPaths []string
	cmd := exec.Command("where", "gcc")
	if output, err := cmd.Output(); err == nil {
		gccPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, gccPath := range gccPaths {
			gccPath = strings.TrimSpace(gccPath)
			if gccPath != "" {
				gccDir := filepath.Dir(gccPath)
				mingwRoot := filepath.Dir(gccDir)
				searchPaths = append(searchPaths, mingwRoot)
			}
		}
	}
	for _, gccVariant := range []string{"mingw32-gcc", "x86_64-w64-mingw32-gcc", "i686-w64-mingw32-gcc"} {
		cmd := exec.Command("where", gccVariant)
		if output, err := cmd.Output(); err == nil {
			gccPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, gccPath := range gccPaths {
				gccPath = strings.TrimSpace(gccPath)
				if gccPath != "" {
					gccDir := filepath.Dir(gccPath)
					mingwRoot := filepath.Dir(gccDir)
					searchPaths = append(searchPaths, mingwRoot)
				}
			}
		}
	}
	commonPaths := []string{
		"C:/msys64/mingw64",
		"C:/msys64/ucrt64",
		"C:/Users/" + os.Getenv("USERNAME") + "/scoop/apps/msys2/current/mingw64",
		"C:/MinGW",
		"C:/mingw64",
		"C:/TDM-GCC-64",
	}

	if mingwHome := os.Getenv("MINGW_HOME"); mingwHome != "" {
		searchPaths = append([]string{mingwHome}, searchPaths...)
	}
	if msys2Root := os.Getenv("MSYSTEM_PREFIX"); msys2Root != "" {
		searchPaths = append([]string{msys2Root}, searchPaths...)
	}
	searchPaths = append(searchPaths, commonPaths...)
	for _, basePath := range searchPaths {
		incPath := filepath.Join(basePath, "include", "gc.h")
		libPath := filepath.Join(basePath, "lib", "libgc.a")

		if _, err := os.Stat(incPath); err == nil {
			if _, err := os.Stat(libPath); err == nil {
				return []string{
					"-I", filepath.Join(basePath, "include"),
					"-L", filepath.Join(basePath, "lib"),
					"-lgc",
				}
			}
		}
	}

	return nil
}
