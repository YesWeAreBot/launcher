[CmdletBinding()]
param(
    [Parameter()]
    [string]$Channel = 'nightly',
    [Parameter()]
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'YesImBot\bin')
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'


if ($Channel -notmatch '^[A-Za-z0-9._-]+$') {
    throw "Invalid channel: $Channel"
}
if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    throw 'Install directory must not be empty'
}

$architectureName = if ($env:PROCESSOR_ARCHITEW6432) {
    $env:PROCESSOR_ARCHITEW6432
} else {
    $env:PROCESSOR_ARCHITECTURE
}
$arch = switch ($architectureName.ToUpperInvariant()) {
    'AMD64' { 'amd64'; break }
    'ARM64' { 'arm64'; break }
    default { throw "Unsupported CPU architecture: $architectureName" }
}

$asset = "yesimbot-cli-windows-$arch.exe"
$url = "https://github.com/YesWeAreBot/launcher/releases/download/$Channel/$asset"
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
$InstallDir = (Resolve-Path -LiteralPath $InstallDir).Path
$target = Join-Path $InstallDir 'yesimbot-cli.exe'
$tempPath = Join-Path $InstallDir ".yesimbot-cli.$([guid]::NewGuid()).tmp"

try {
    Write-Host "Downloading $Channel (windows/$arch) from YesWeAreBot/launcher"
    Invoke-WebRequest -Uri $url -OutFile $tempPath -UseBasicParsing
    Move-Item -LiteralPath $tempPath -Destination $target -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $currentPath = $env:Path
    $hasInstallDir = @($currentPath, $userPath) | ForEach-Object {
        if ([string]::IsNullOrEmpty($_)) {
            $false
        } else {
            $_ -split ';' | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') } | Select-Object -First 1
        }
    } | Where-Object { $_ } | Select-Object -First 1

    if ($hasInstallDir) {
        $pathStatus = 'PATH already contains the install directory'
    } else {
        $newUserPath = if ([string]::IsNullOrEmpty($userPath)) {
            $InstallDir
        } else {
            "$userPath;$InstallDir"
        }
        [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
        $pathStatus = "added $InstallDir to the current user's PATH"
    }

    if (@($env:Path -split ';' | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') }).Count -eq 0) {
        $env:Path = "$InstallDir;$env:Path"
    }

    Write-Host "Installed $Channel to $target"
    Write-Host $pathStatus
    Write-Host ''
    Write-Host '--- yesimbot-cli --help ---'
    & $target --help
    if ($LASTEXITCODE -ne 0) {
        throw "yesimbot-cli --help failed with exit code $LASTEXITCODE"
    }
} catch {
    Write-Error $_
    exit 1
} finally {
    if ($tempPath -and (Test-Path -LiteralPath $tempPath)) {
        Remove-Item -LiteralPath $tempPath -Force -ErrorAction SilentlyContinue
    }
}
