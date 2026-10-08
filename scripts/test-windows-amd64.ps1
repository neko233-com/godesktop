param(
    [ValidateRange(1, 20)][int]$Repeat = 3,
    [ValidateRange(1, 300)][int]$FuzzSeconds = 5
)

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) { throw 'This script requires Windows.' }
if (-not [Environment]::Is64BitOperatingSystem) { throw 'A 64-bit Windows OS is required.' }
foreach ($taskTool in @('go', 'gcc', 'g++')) {
    if (-not (Get-Command $taskTool -ErrorAction SilentlyContinue)) { throw "$taskTool must be available in PATH." }
}
$taskCompiler = (& gcc -dumpmachine).Trim()
if ($LASTEXITCODE -ne 0 -or $taskCompiler -notmatch '^x86_64-.*mingw') { throw "Expected x86-64 MinGW-w64, found $taskCompiler" }

function Invoke-CheckedGo {
    & go @args
    if ($LASTEXITCODE -ne 0) { throw "go $args failed with exit code $LASTEXITCODE" }
}

$taskRoot = Split-Path -Parent $PSScriptRoot
$taskEnvNames = @('GOOS', 'GOARCH', 'GOAMD64', 'CGO_ENABLED', 'GOEXPERIMENT', 'CC', 'CXX', 'GODESKTOP_NATIVE_COVERDIR', 'GODESKTOP_TEST_INPUT_ISOLATION')
$taskSavedEnv = @{}
foreach ($taskName in $taskEnvNames) { $taskSavedEnv[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
Push-Location -LiteralPath $taskRoot
try {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:GOAMD64 = 'v1'
    $env:CGO_ENABLED = '1'
    $env:GOEXPERIMENT = 'cgocheck2'
    $env:CC = 'gcc'
    $env:CXX = 'g++'
    $env:GODESKTOP_TEST_INPUT_ISOLATION = '1'
    $taskReportDir = [IO.Path]::GetFullPath((Join-Path $taskRoot '.cache/windows-validation/current'))
    $taskExpectedParent = [IO.Path]::GetFullPath((Join-Path $taskRoot '.cache/windows-validation'))
    if ([IO.Path]::GetDirectoryName($taskReportDir) -ne $taskExpectedParent -or [IO.Path]::GetFileName($taskReportDir) -ne 'current') { throw 'Invalid owned validation report directory.' }
    if (Test-Path -LiteralPath $taskReportDir) { Remove-Item -LiteralPath $taskReportDir -Recurse -Force }
    $taskCoreDir = Join-Path $taskReportDir 'core'
    $taskNativeDir = Join-Path $taskReportDir 'native'
    New-Item -ItemType Directory -Path $taskCoreDir, $taskNativeDir -Force | Out-Null
    $env:GODESKTOP_NATIVE_COVERDIR = $taskNativeDir
    Write-Output "Windows amd64 / GOAMD64=v1 / compiler=$taskCompiler / strict cgo checking"
    Invoke-CheckedGo version
    # Packages share native/root coverage metadata; Windows atomic replacement
    # must not race another package emitting the same metadata filename.
    Invoke-CheckedGo test -p=1 -race -shuffle=on "-count=$Repeat" -timeout=8m '-coverpkg=github.com/neko233-com/godesktop,github.com/neko233-com/godesktop/internal/platform' "-coverprofile=$taskReportDir/core.out" ./... -args "-test.gocoverdir=$taskCoreDir"
    Invoke-CheckedGo vet ./...
    foreach ($taskFuzz in @('FuzzRectIntersection', 'FuzzLayoutClipsToViewport')) {
        Invoke-CheckedGo test -run '^$' -fuzz "^${taskFuzz}$" "-fuzztime=${FuzzSeconds}s" -parallel=4 .
    }
    Invoke-CheckedGo test -run '^$' -fuzz '^FuzzBufferTransactions$' "-fuzztime=${FuzzSeconds}s" -parallel=4 ./editor
    $taskCoverageInputs = "$taskCoreDir,$taskNativeDir"
    Invoke-CheckedGo tool covdata textfmt "-i=$taskCoverageInputs" "-o=$taskReportDir/coverage.out"
    $taskCoverage = & go tool covdata percent "-i=$taskCoverageInputs"
    if ($LASTEXITCODE -ne 0) { throw 'Coverage merge failed.' }
    $taskCoverage | Tee-Object -FilePath (Join-Path $taskReportDir 'coverage.txt')
    $taskCoveragePattern = '^\s*github\.com/neko233-com/godesktop\s+coverage:\s+([0-9.]+)%'
    $taskRootCoverage = $taskCoverage | Where-Object { $_ -match $taskCoveragePattern } | Select-Object -First 1
    if (-not $taskRootCoverage) { throw 'Root-package coverage is missing.' }
    $taskCoverageMatch = [regex]::Match($taskRootCoverage, $taskCoveragePattern)
    if ([double]::Parse($taskCoverageMatch.Groups[1].Value, [Globalization.CultureInfo]::InvariantCulture) -lt 90) { throw 'Combined root-package coverage must be at least 90%.' }
    New-Item -ItemType Directory -Path bin -Force | Out-Null
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w' -o bin/counter.exe ./examples/counter
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w -H=windowsgui' -o bin/counter-gui.exe ./examples/counter
    Invoke-CheckedGo run ./internal/pecheck bin/counter.exe
    Invoke-CheckedGo run ./internal/pecheck bin/counter-gui.exe
    & .\bin\counter.exe -smoke
    if ($LASTEXITCODE -ne 0) { throw 'Console EXE smoke failed.' }
    Invoke-CheckedGo run ./internal/guismoke ./bin/counter-gui.exe
    Write-Output "Windows amd64 validation passed. Reports: $taskReportDir"
} finally {
    foreach ($taskName in $taskEnvNames) { [Environment]::SetEnvironmentVariable($taskName, $taskSavedEnv[$taskName], 'Process') }
    Pop-Location
}
