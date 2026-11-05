param (
    [string]$CoverageFile = "coverage.out",
    [double]$Threshold = 60.0
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $CoverageFile)) {
    Write-Error "Coverage profile '$CoverageFile' not found!"
    exit 1
}

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "    MS1 (jira-issues-go) Statement Coverage Verification    " -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

$hasGo = Get-Command go -ErrorAction SilentlyContinue
if ($hasGo) {
    $coverFuncOutput = go tool cover -func=$CoverageFile
} elseif (Get-Command docker -ErrorAction SilentlyContinue) {
    $fileName = Split-Path -Leaf $CoverageFile
    $coverFuncOutput = docker run --rm -v "C:\jira-ai:/jira-ai" -w /jira-ai/jira-issues-go golang:1.22-alpine go tool cover "-func=$fileName"
} else {
    Write-Error "Neither Go CLI nor Docker is available to calculate coverage."
    exit 1
}

$totalLine = $coverFuncOutput | Where-Object { $_ -match "total:\s+\(statements\)\s+([0-9.]+)%" }

if (-not $totalLine) {
    Write-Error "Failed to extract total statement coverage from $CoverageFile"
    exit 1
}

$coverageStr = $Matches[1]
$coverage = [double]$coverageStr

Write-Host "Current Statement Coverage : $coverage%"
Write-Host "Required Minimum Threshold : $Threshold%"
Write-Host "------------------------------------------------------------"

if ($coverage -ge $Threshold) {
    Write-Host "SUCCESS: Statement coverage threshold ($Threshold%) satisfied!" -ForegroundColor Green
    exit 0
} else {
    Write-Error "FAILURE: Statement coverage ($coverage%) is below required threshold ($Threshold%)!"
    exit 1
}
