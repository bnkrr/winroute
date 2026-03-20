param(
    [Parameter(Mandatory = $true)]
    [string]$WRoutePath,

    [Parameter(Mandatory = $true)]
    [uint32]$InterfaceIndex,

    [Parameter(Mandatory = $true)]
    [string]$NextHop,

    [string]$DestinationPrefix = "198.19.250.0/24"
)

$ErrorActionPreference = "Stop"

function Assert-Admin {
    $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Please run this script from an elevated PowerShell session."
    }
}

function Assert-CommandSucceeded {
    param(
        [int]$ExitCode,
        [string]$Action,
        [string]$Output
    )

    if ($ExitCode -ne 0) {
        throw "$Action failed with exit code $ExitCode.`n$Output"
    }
}

function Run-WRoute {
    param(
        [string[]]$Arguments,
        [switch]$AllowFailure
    )

    $output = & $script:WRoutePath @Arguments 2>&1
    $exitCode = $LASTEXITCODE
    if (-not $AllowFailure) {
        Assert-CommandSucceeded -ExitCode $exitCode -Action ("wroute " + ($Arguments -join " ")) -Output ($output | Out-String)
    }

    [pscustomobject]@{
        ExitCode = $exitCode
        Output   = ($output | Out-String).Trim()
    }
}

function Test-RouteAbsentOutput {
    param([string]$Output)

    return [string]::IsNullOrWhiteSpace($Output) -or $Output -eq "No routes found matching the criteria."
}

function Assert-RouteAbsent {
    param([string]$Prefix)

    $result = Run-WRoute -Arguments @("get", "--destination", $Prefix)
    if (-not (Test-RouteAbsentOutput -Output $result.Output)) {
        throw "Expected route $Prefix to be absent.`n$($result.Output)"
    }
}

function Assert-RoutePresent {
    param([string]$Prefix)

    $result = Run-WRoute -Arguments @("get", "--destination", $Prefix)
    if ($result.Output -notmatch [Regex]::Escape($Prefix)) {
        throw "Expected route $Prefix to be present.`n$($result.Output)"
    }
}

Assert-Admin

$repoRoot = Split-Path -Parent $PSScriptRoot
$script:WRoutePath = (Resolve-Path $WRoutePath).Path

Push-Location $repoRoot
try {
    Write-Host "Inspecting initial route state for $DestinationPrefix ..."
    Assert-RouteAbsent -Prefix $DestinationPrefix

    Write-Host "Adding test route for delete-one ..."
    Run-WRoute -Arguments @(
        "add",
        "--destination", $DestinationPrefix,
        "--next-hop", $NextHop,
        "--if-index", "$InterfaceIndex",
        "--metric", "777"
    ) | Out-Null

    Write-Host "Verifying route was added ..."
    Assert-RoutePresent -Prefix $DestinationPrefix

    Write-Host "Deleting the test route with delete-one ..."
    Run-WRoute -Arguments @(
        "delete-one",
        "--destination", $DestinationPrefix,
        "--next-hop", $NextHop,
        "--if-index", "$InterfaceIndex"
    ) | Out-Null

    Write-Host "Verifying route was removed ..."
    Assert-RouteAbsent -Prefix $DestinationPrefix

    Write-Host "Adding test route for filtered delete ..."
    Run-WRoute -Arguments @(
        "add",
        "--destination", $DestinationPrefix,
        "--next-hop", $NextHop,
        "--if-index", "$InterfaceIndex",
        "--metric", "777"
    ) | Out-Null

    Write-Host "Verifying route was added again ..."
    Assert-RoutePresent -Prefix $DestinationPrefix

    Write-Host "Deleting the test route with delete ..."
    Run-WRoute -Arguments @(
        "delete",
        "--destination", $DestinationPrefix,
        "--if-index", "$InterfaceIndex"
    ) | Out-Null

    Write-Host "Verifying route was removed again ..."
    Assert-RouteAbsent -Prefix $DestinationPrefix

    Write-Host "Integration test passed."
}
finally {
    Pop-Location
}
