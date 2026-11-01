<#
.SYNOPSIS
  Makes the Windows installer (.msi) from the folder scripts/build-windows.sh built.
.DESCRIPTION
  Needs the WiX Toolset, on the PATH or at $env:WIX_EXE, with its two extensions:
    dotnet tool install --tool-path <folder> wix --version 6.0.2
    wix extension add -g WixToolset.UI.wixext/6.0.2 WixToolset.Util.wixext/6.0.2
  and Go, which draws the wizard's pictures.
.EXAMPLE
  packaging\windows\build-msi.ps1 -Version 0.1.1 -PackageDir .build\WittyMediaPlayer-0.1.1-windows-amd64 -OutDir .build
#>
param(
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$PackageDir,
  [string]$OutDir = ".build"
)
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$root = (Resolve-Path (Join-Path $here "..\..")).Path
$wix = if ($env:WIX_EXE) { $env:WIX_EXE } elseif (Get-Command wix -ErrorAction SilentlyContinue) { "wix" } else { throw "The WiX Toolset is not installed: see the top of this script." }
$PackageDir = (Resolve-Path $PackageDir).Path
New-Item -ItemType Directory -Force $OutDir | Out-Null
$OutDir = (Resolve-Path $OutDir).Path

# Windows Installer versions are three numbers; what's after them is for people to read.
$numeric = ($Version -replace '^(\d+\.\d+\.\d+).*$', '$1')
if ($numeric -notmatch '^\d+\.\d+\.\d+$') { throw "$Version doesn't start with three numbers" }

foreach ($need in "WittyMediaPlayer.exe", "Uninstall WittyMediaPlayer.exe", "package-files.txt", "bin\wittymediaplayer-engine.exe", "bin\wittymediaplayer-updater.exe") {
  if (-not (Test-Path -LiteralPath (Join-Path $PackageDir $need))) { throw "$need isn't in $PackageDir" }
}

$work = Join-Path $OutDir (".msi-work-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Force $work | Out-Null
try {
  Copy-Item (Join-Path $root "resources\WittyMediaPlayer.ico") $work
  Push-Location (Join-Path $root "engine")
  try {
    $env:GOOS = $null; $env:GOARCH = $null
    go run ./tools/genicon -bitmaps $work
    if ($LASTEXITCODE -ne 0) { throw "making the installer's pictures failed" }
  } finally { Pop-Location }

  # What Installed apps says it takes up.
  $sizeKB = [int64](((Get-ChildItem -LiteralPath $PackageDir -Recurse -File | Measure-Object Length -Sum).Sum) / 1024)
  $out = Join-Path $OutDir "WittyMediaPlayer-$Version-windows-amd64.msi"
  & $wix build `
    (Join-Path $here "WittyMediaPlayer.wxs") (Join-Path $here "WittyUI.wxs") `
    -arch x64 `
    -d "Version=$numeric" -d "DisplayVersion=$Version" -d "SizeKB=$sizeKB" `
    -bindpath "pkg=$PackageDir" -bindpath "res=$work" `
    -ext WixToolset.UI.wixext -ext WixToolset.Util.wixext `
    -intermediatefolder (Join-Path $work "intermediate") `
    -o $out
  if ($LASTEXITCODE -ne 0) { throw "wix build failed" }
  Remove-Item -Force -ErrorAction SilentlyContinue ($out -replace '\.msi$', '.wixpdb')
  Write-Host "   -> $out"
} finally {
  Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $work
}
