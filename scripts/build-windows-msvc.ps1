[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Preset
)

$ErrorActionPreference = 'Stop'

$vswherePath = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
if (-not (Test-Path -LiteralPath $vswherePath -PathType Leaf)) {
    throw "vswhere.exe was not found at $vswherePath"
}

$vsInstallPath = & $vswherePath -latest -products * `
    -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
    -property installationPath
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($vsInstallPath)) {
    throw 'A Visual Studio installation with the MSVC x64 tools was not found'
}

$devShellModule = Join-Path $vsInstallPath 'Common7\Tools\Microsoft.VisualStudio.DevShell.dll'
if (-not (Test-Path -LiteralPath $devShellModule -PathType Leaf)) {
    throw "Visual Studio developer shell module was not found at $devShellModule"
}

Import-Module $devShellModule
Enter-VsDevShell -VsInstallPath $vsInstallPath -SkipAutomaticLocation `
    -DevCmdArguments '-arch=x64 -host_arch=x64'

$compiler = Get-Command cl.exe -CommandType Application -ErrorAction Stop
Write-Host "Using MSVC compiler: $($compiler.Source)"
$env:CC = 'cl.exe'
$env:CXX = 'cl.exe'

python scripts/retry-cmake-configure.py cmake --preset $Preset `
    -DCMAKE_BUILD_TYPE=Release `
    -DTOGGLE_BIN_FOLDER=ON `
    -DOPTIONS_ENABLE_IPO=ON `
    -DOTCLIENT_BUILD_TESTS=OFF
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

cmake --build --preset $Preset --config Release --target otclient
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
