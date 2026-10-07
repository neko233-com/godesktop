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
    $taskWorkflows = @(Get-ChildItem -LiteralPath '.github/workflows' -File |
        Where-Object { $_.Extension -in @('.yml', '.yaml') } |
        ForEach-Object { $_.FullName })
    if ($taskWorkflows.Count -eq 0) { throw 'No GitHub Actions workflows found.' }
    actionlint @taskWorkflows
    if ($LASTEXITCODE -ne 0) {
        throw "actionlint failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}
