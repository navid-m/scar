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

set "MINGW64_BIN=%SCOOP%\apps\msys2\current\mingw64\bin"

for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "CURRENT_PATH=%%b"
if not defined CURRENT_PATH set "CURRENT_PATH="

echo !CURRENT_PATH! | findstr /i /c:"%MINGW64_BIN%" >nul
if %errorlevel% neq 0 (
    echo Adding %MINGW64_BIN% to user PATH...
    if defined CURRENT_PATH (
        set "NEW_PATH=!CURRENT_PATH!;%MINGW64_BIN%"
    ) else (
        set "NEW_PATH=%MINGW64_BIN%"
    )
    reg add "HKCU\Environment" /v PATH /t REG_EXPAND_SZ /d "!NEW_PATH!" /f >nul
    echo You may need to restart your command prompt or IDE to see the changes.
) else (
    echo GCC already in PATH.
)

echo Done.
pause