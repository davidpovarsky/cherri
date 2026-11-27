<#
.SYNOPSIS
    Checks that required named AI team files are present.
.DESCRIPTION
    Read-only file-presence precheck for the current worktree. This does not
    validate file contents, configuration integrity, or runtime activation.
    Complete the separate checks in docs/ai-team/VALIDATION.md.
.PARAMETER Check
    Performs the file-presence precheck (default).
.PARAMETER Apply
    Unsupported. Exits with an error without modifying any files.
#>
[CmdletBinding()]
param(
    [switch]$Check = $true,
    [switch]$Apply = $false
)

$ErrorActionPreference = "Stop"

if ($Apply) {
    Write-Host "FAIL: -Apply is unsupported; no files were changed." -ForegroundColor Red
    exit 2
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

Write-Host "Checking AI Team configuration in: $RepoRoot" -ForegroundColor Cyan

$RequiredFiles = @(
    "AGENTS.md",
    "GEMINI.md",
    ".agents\rules\ai-team-router.md",
    ".agents\skills\asana-team-coordination\SKILL.md",
    "docs\ai-team\TEAM.md",
    "docs\ai-team\PROJECT_ASSIGNMENTS.md",
    "docs\ai-team\ASANA_POLICY.md",
    "docs\ai-team\TASK_HANDOFF_TEMPLATE.md",
    "docs\ai-team\PLATFORMS.md",
    "docs\ai-team\BRANCH_INTEGRATION.md",
    "docs\ai-team\VALIDATION.md"
)

$Roles = @("eitan", "noam", "maya", "roni", "tomer", "yael", "dana")
foreach ($r in $Roles) {
    $RequiredFiles += ".agents\agents\$r.md"
    $RequiredFiles += ".agents\roles\$r.md"
    $RequiredFiles += ".codex\agents\$r.toml"
}

$Missing = @()
foreach ($f in $RequiredFiles) {
    $fullPath = Join-Path $RepoRoot $f
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        $Missing += $f
    }
}

if ($Missing.Count -eq 0) {
    Write-Host "PASS: All $($RequiredFiles.Count) required team files are present (presence precheck only)." -ForegroundColor Green
    exit 0
} else {
    Write-Host "FAIL: $($Missing.Count) required paths are missing or are not files:" -ForegroundColor Red
    foreach ($m in $Missing) {
        Write-Host "  - $m" -ForegroundColor Red
    }
    exit 1
}
