@echo off
setlocal enabledelayedexpansion

where scoop >nul 2>&1
if %errorlevel% neq 0 (
    echo Scoop not found - installing...
    powershell -Command "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force"
    powershell -Command "$scoopInstaller = Invoke-WebRequest -Uri 'https://get.scoop.sh' -UseBasicParsing; Invoke-Expression $scoopInstaller.Content"
    call refreshenv.exe 2>nul || (
        for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "USERPATH=%%b"
        if defined USERPATH set "PATH=%PATH%;%USERPATH%"
        set "PATH=%PATH%;%USERPROFILE%\scoop\shims"
    )
) else (
    echo Scoop already installed.
)
if not defined SCOOP set "SCOOP=%USERPROFILE%\scoop"
if not exist "%SCOOP%\apps\msys2" (
    echo Installing MSYS2 ^(with MinGW64^) via Scoop...
    scoop install msys2
) else (
    echo MSYS2 already installed.
)

echo Updating MSYS2...
scoop update msys2
set "MSYS2_SHELL=%SCOOP%\apps\msys2\current\usr\bin\bash.exe"
if not exist "%MSYS2_SHELL%" (
    echo Error: MSYS2 bash not found at %MSYS2_SHELL%
    pause
    exit /b 1
)

echo Installing MinGW64 toolchain and libraries...

echo Updating package database...
"%MSYS2_SHELL%" -lc "pacman --noconfirm -Syu"

echo Installing toolchain...
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-toolchain"

echo Installing additional libraries...
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-curl mingw-w64-x86_64-pcre mingw-w64-x86_64-jansson"

echo Done.
pause