$ErrorActionPreference = 'Stop'

if (-not (Get-Command actionlint -ErrorAction SilentlyContinue)) {
    throw 'actionlint must be installed and available in PATH.'
}
if (-not (Get-Command shellcheck -ErrorAction SilentlyContinue)) {
    throw 'ShellCheck must be installed and available in PATH.'
}

$taskRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $taskRoot
try {
    actionlint .github/workflows/ci.yml
    if ($LASTEXITCODE -ne 0) {
        throw "actionlint failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}
