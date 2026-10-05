param([switch]$Regenerate)
$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitProcess) { throw 'Shader verification requires a 64-bit PowerShell process.' }
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskCompilerRoot = Join-Path $taskRoot '.cache/tools/dxc-1.9.2609'
$taskCompiler = Join-Path $taskCompilerRoot 'bin/x64/dxc.exe'
if (-not (Test-Path -LiteralPath $taskCompiler)) {
    New-Item -ItemType Directory -Path $taskCompilerRoot -Force | Out-Null
    $taskArchive = Join-Path $taskCompilerRoot 'dxc.zip'
    Invoke-WebRequest -Uri 'https://github.com/microsoft/DirectXShaderCompiler/releases/download/v1.9.2609/dxc_2026_09_29.zip' -OutFile $taskArchive
    $taskDigest = (Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($taskDigest -ne 'ad31b1fc8443175d204f77a611fdb3ef2ec42759bdc2f1167368de24a4a7e7f1') { throw 'DXC archive SHA256 mismatch.' }
    Expand-Archive -LiteralPath $taskArchive -DestinationPath $taskCompilerRoot -Force
}
Push-Location -LiteralPath $taskRoot
try {
    $taskCompilerArgs = @('./internal/shadergen','-dxc',$taskCompiler)
    if (-not $Regenerate) { $taskCompilerArgs += '-check' }
    & go run @taskCompilerArgs
    if ($LASTEXITCODE -ne 0) { throw "Shader verification failed with exit code $LASTEXITCODE." }
} finally {
    Pop-Location
}
