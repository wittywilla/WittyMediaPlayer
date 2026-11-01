<#
.SYNOPSIS
  Runs the real Uninstall WittyMediaPlayer.exe on real Windows against a copy of the package and a
  made-up account: every folder it looks in is pointed at a scratch folder first, so the only thing
  it can reach on this computer is the registry, where it finds nothing of ours unless the player is
  really installed. In that case this refuses to run. It uses /S, so it opens no window.
.EXAMPLE
  scripts\test\uninstall-portable.ps1 -Package .build\WittyMediaPlayer-0.1.1-windows-amd64 -Scratch D:\scratch\uninstall
#>
param(
  [Parameter(Mandatory)][string]$Package,
  [Parameter(Mandatory)][string]$Scratch
)
$ErrorActionPreference = "Stop"
foreach ($k in "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WittyMediaPlayer", "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WittyMediaPlayer", "HKCU:\Software\WittyWilla\WittyMediaPlayer", "HKLM:\Software\WittyWilla\WittyMediaPlayer") {
  if (Test-Path $k) { throw "WittyMediaPlayer is really installed here ($k), so this won't run." }
}
if (Test-Path $Scratch) { Remove-Item -Recurse -Force $Scratch }
$failed = 0
function Fail($text) { Write-Host "FAIL: $text"; $script:failed++ }

$home2 = Join-Path $Scratch "Users\me"
$local = Join-Path $home2 "AppData\Local"
$roaming = Join-Path $home2 "AppData\Roaming"
$tools = Join-Path $Scratch "Tools\WittyMediaPlayer"
New-Item -ItemType Directory -Force $local, $roaming, (Join-Path $home2 "Temp"), (Join-Path $home2 "Desktop"), (Join-Path $Scratch "Tools") | Out-Null
Copy-Item -Recurse $Package $tools

function Mine($path, $text = "x") {
  New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
  Set-Content -LiteralPath $path -Value $text
}
$ini = Join-Path $roaming "WittyMediaPlayer\WittyMediaPlayer.ini"
$state = Join-Path $local "WittyMediaPlayer\modules\wittydvd\state.json"
$notes = Join-Path $local "WittyMediaPlayer\my-notes.txt"
$film = Join-Path $home2 "Videos\A Made-Up Film.mkv"
$album = Join-Path $home2 "Music\A Made-Up Album\01.flac"
$own = Join-Path $tools "my-own-file.txt"
$desk = Join-Path $home2 "Desktop\WittyMediaPlayer.lnk"
$ini, $state, $notes, $film, $album, $own, $desk | ForEach-Object { Mine $_ }

# Everything it looks at, in the made-up account.
foreach ($v in @{
    APPDATA = $roaming; LOCALAPPDATA = $local; USERPROFILE = $home2; TEMP = (Join-Path $home2 "Temp"); TMP = (Join-Path $home2 "Temp")
    PUBLIC = (Join-Path $Scratch "Users\Public"); ProgramData = (Join-Path $Scratch "ProgramData"); ProgramFiles = (Join-Path $Scratch "Program Files")
  }.GetEnumerator()) { Set-Item -Path "Env:$($v.Key)" -Value $v.Value }

$un = Join-Path $tools "Uninstall WittyMediaPlayer.exe"
$dry = Join-Path $Scratch "dry.txt"
$p = Start-Process -FilePath $un -ArgumentList @("--dry-run", "--report", "`"$dry`"") -Wait -PassThru
Write-Host "--dry-run exit code: $($p.ExitCode)"
Get-Content $dry
if ((Get-Item -LiteralPath $ini).Length -eq 0) { Fail "the dry run touched something" }

$report = Join-Path $Scratch "report.txt"
$started = Get-Date
$p = Start-Process -FilePath $un -ArgumentList @("/S", "/RemoveData", "--report", "`"$report`"") -Wait -PassThru
Write-Host "/S /RemoveData exit code: $($p.ExitCode), back in $([int]((Get-Date) - $started).TotalMilliseconds) ms"
$deadline = (Get-Date).AddSeconds(60)
while (-not (Test-Path $report) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 200 }
if (-not (Test-Path $report)) { Fail "no report after a minute"; exit 1 }
Write-Host "finished after $([int]((Get-Date) - $started).TotalMilliseconds) ms"
Get-Content $report
Start-Sleep -Seconds 6

if (Test-Path -LiteralPath (Join-Path $tools "WittyMediaPlayer.exe")) { Fail "the program stayed" }
if (Test-Path -LiteralPath (Join-Path $tools "bin")) { Fail "bin stayed" }
if (Test-Path -LiteralPath (Join-Path $tools "platforms")) { Fail "the plugins stayed" }
if (Test-Path -LiteralPath $un) { Fail "the uninstaller couldn't remove itself" }
if (-not (Test-Path -LiteralPath $own)) { Fail "it took a file it didn't make" }
if (Test-Path -LiteralPath $ini) { Fail "the settings stayed" }
if (Test-Path -LiteralPath (Split-Path $ini)) { Fail "the settings folder stayed" }
if (Test-Path -LiteralPath (Join-Path $local "WittyMediaPlayer\modules")) { Fail "the modules stayed" }
if (-not (Test-Path -LiteralPath $notes)) { Fail "it took a file it didn't make, from the data folder" }
if (Test-Path -LiteralPath $desk) { Fail "the shortcut stayed" }
if (-not (Test-Path -LiteralPath $film)) { Fail "A FILM WAS DELETED" }
if (-not (Test-Path -LiteralPath $album)) { Fail "A COPIED CD WAS DELETED" }
if (Get-ChildItem (Join-Path $home2 "Temp") -Filter "wittymediaplayer-uninstall-*" -ErrorAction SilentlyContinue) { Fail "the worker's copy stayed in the temp folder" }
if (-not (Select-String -Path $report -SimpleMatch "my-own-file.txt" -Quiet)) { Fail "the report doesn't say it left a file" }

if ($failed) { Write-Host "FAILED: $failed"; exit 1 }
Write-Host "ALLPASSED"
