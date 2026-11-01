<#
.SYNOPSIS
  Checks a built MSI without installing it: reads its tables, unpacks it with an administrative
  extract, which registers nothing, and checks what comes out. Needs no administrator, opens no
  window, and changes nothing on this computer but the folder it's given.
.EXAMPLE
  scripts\test\check-msi.ps1 -Msi .build\WittyMediaPlayer-0.1.1-windows-amd64.msi -Version 0.1.1 -Scratch D:\scratch\msi-check
#>
param(
  [Parameter(Mandatory)][string]$Msi,
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$Scratch,
  [string]$PackageDir = ""
)
$ErrorActionPreference = "Stop"
$Msi = (Resolve-Path $Msi).Path
# Windows Installer wants a full path, with backslashes.
$Scratch = (New-Item -ItemType Directory -Force $Scratch).FullName
$failed = 0
function Fail($text) { Write-Host "FAIL: $text"; $script:failed++ }
function Ok($text) { Write-Host "ok: $text" }

function Query($db, $sql, $cols) {
  $view = $db.GetType().InvokeMember("OpenView", "InvokeMethod", $null, $db, @($sql))
  [void]$view.GetType().InvokeMember("Execute", "InvokeMethod", $null, $view, $null)
  $rows = @()
  while ($true) {
    $rec = $view.GetType().InvokeMember("Fetch", "InvokeMethod", $null, $view, $null)
    if ($null -eq $rec) { break }
    $row = @{}
    for ($i = 0; $i -lt $cols.Count; $i++) {
      $row[$cols[$i]] = $rec.GetType().InvokeMember("StringData", "GetProperty", $null, $rec, @($i + 1))
    }
    $rows += [pscustomobject]$row
  }
  [void]$view.GetType().InvokeMember("Close", "InvokeMethod", $null, $view, $null)
  return $rows
}

$installer = New-Object -ComObject WindowsInstaller.Installer
$db = $installer.GetType().InvokeMember("OpenDatabase", "InvokeMethod", $null, $installer, @($Msi, 0))

# ---- the tables
$props = @{}
foreach ($r in (Query $db 'SELECT Property, Value FROM Property' @('p', 'v'))) { $props[$r.p] = $r.v }
Write-Host "ProductName=$($props.ProductName) ProductVersion=$($props.ProductVersion) UpgradeCode=$($props.UpgradeCode) ProductCode=$($props.ProductCode)"
Write-Host "ALLUSERS=$($props.ALLUSERS) MSIINSTALLPERUSER=$($props.MSIINSTALLPERUSER) ARPSYSTEMCOMPONENT=$($props.ARPSYSTEMCOMPONENT)"
if ($props.ProductName -ne "WittyMediaPlayer") { Fail "product name" }
if ($props.UpgradeCode -ne "{FF1C58FA-DA0C-4E9C-9D4D-B0A7619D33B1}") { Fail "the upgrade code changed: $($props.UpgradeCode)" } else { Ok "the upgrade code is the fixed one" }
if ($props.ALLUSERS -ne "2" -or $props.MSIINSTALLPERUSER -ne "1") { Fail "it should default to just this account, and be able to install for everyone" } else { Ok "per-user by default, per-machine when asked" }
if ($props.ARPSYSTEMCOMPONENT -ne "1") { Fail "the MSI's own line would show in Installed apps" }
foreach ($p in "ASSOCVIDEO", "ASSOCMUSIC", "ASSOCDISC") { if ($props[$p] -ne "1") { Fail "$p should start on" } }
foreach ($p in "DESKTOPSHORTCUT", "ASSOCVIDEO", "ASSOCMUSIC", "ASSOCDISC") { if ($props.SecureCustomProperties -notmatch $p) { Fail "$p isn't secure, so a wizard's answer wouldn't reach the install" } }

# Windows Installer's own flags. 8 would mean it never asks for an administrator, which would break
# an install for everyone; 2 means the files are inside the .msi.
$summary = $db.GetType().InvokeMember("SummaryInformation", "GetProperty", $null, $db, @(0))
$word = $summary.GetType().InvokeMember("Property", "GetProperty", $null, $summary, @(15))
Write-Host "Summary word count: $word"
if ($word -band 8) { Fail "the package says it never needs an administrator" } else { Ok "it can ask for an administrator when it's for everyone" }
[void][Runtime.InteropServices.Marshal]::ReleaseComObject($summary)

$upgrades = Query $db 'SELECT UpgradeCode, VersionMin, VersionMax, Attributes, ActionProperty FROM Upgrade' @('code', 'min', 'max', 'attr', 'prop')
$upgrades | ForEach-Object { Write-Host "Upgrade: $($_.code) min=$($_.min) max=$($_.max) attr=$($_.attr) -> $($_.prop)" }
if (-not ($upgrades | Where-Object { $_.prop -eq 'WIX_UPGRADE_DETECTED' })) { Fail "nothing upgrades in place" }

$shortcuts = Query $db 'SELECT Shortcut, Directory_, Name, Component_, Target FROM Shortcut' @('id', 'dir', 'name', 'comp', 'target')
$shortcuts | ForEach-Object { Write-Host "Shortcut: $($_.name) in $($_.dir), component $($_.comp), target $($_.target)" }
if (-not ($shortcuts | Where-Object { $_.dir -eq 'ProgramMenuFolder' })) { Fail "no Start menu entry" }
if (-not ($shortcuts | Where-Object { $_.dir -eq 'DesktopFolder' })) { Fail "no desktop shortcut" }

$comps = Query $db 'SELECT Component, Condition FROM Component' @('id', 'cond')
foreach ($want in @{DesktopEntry = 'DESKTOPSHORTCUT'; OpenVideo = 'ASSOCVIDEO'; OpenMusic = 'ASSOCMUSIC'; OpenDisc = 'ASSOCDISC'}.GetEnumerator()) {
  $c = $comps | Where-Object { $_.id -eq $want.Key }
  if (-not $c -or $c.cond -notmatch $want.Value) { Fail "$($want.Key) isn't conditional on $($want.Value)" } else { Ok "$($want.Key) only with $($c.cond)" }
}

$reg = Query $db 'SELECT `Registry`, `Root`, `Key`, `Name`, `Value`, `Component_` FROM `Registry`' @('id', 'root', 'key', 'name', 'value', 'comp')
$arp = $reg | Where-Object { $_.key -like '*CurrentVersion\Uninstall\WittyMediaPlayer' }
Write-Host "Registry rows: $($reg.Count). Installed apps entry:"
$arp | ForEach-Object { Write-Host "  $($_.name) = $($_.value)" }
if (($arp | Where-Object { $_.name -eq 'UninstallString' }).value -notmatch 'Uninstall WittyMediaPlayer\.exe') { Fail "Installed apps doesn't point at our uninstaller" }
if (($arp | Where-Object { $_.name -eq 'QuietUninstallString' }).value -notmatch '/S') { Fail "no quiet uninstall string" }
if (($arp | Where-Object { $_.name -eq 'DisplayVersion' }).value -ne $Version) { Fail "the version shown isn't $Version" }
if ($reg | Where-Object { $_.root -notin '-1' }) { Fail "some registry rows aren't in the install's own hive"; $reg | Where-Object { $_.root -notin '-1' } | ForEach-Object { Write-Host "  $($_.root) $($_.key)" } } else { Ok "every registry row follows per-user or per-machine (HKMU)" }
foreach ($bad in '^Software\\Classes\\\.[^\\]+$') {
  if ($reg | Where-Object { $_.key -match $bad }) { Fail "something takes over a file type's default" } else { Ok "no file type's default is taken over: only Open with lists" }
}
foreach ($type in "WittyMediaPlayer.Video", "WittyMediaPlayer.Music", "WittyMediaPlayer.Disc") {
  $n = ($reg | Where-Object { $_.key -like '*\OpenWithProgids' -and $_.name -eq $type }).Count
  $cmd = ($reg | Where-Object { $_.key -eq "Software\Classes\$type\shell\open\command" }).value
  Write-Host "$type : $n file types; opens with $cmd"
  if ($n -lt 5) { Fail "$type has only $n file types" }
  if ($cmd -notmatch '"%1"') { Fail "$type's command doesn't quote the file" }
}

$files = Query $db 'SELECT File, Component_, FileName, FileSize FROM File' @('id', 'comp', 'name', 'size')
Write-Host "Files in the MSI: $($files.Count)"
$actions = Query $db 'SELECT Action, Condition, Sequence FROM InstallExecuteSequence' @('a', 'c', 's') | Sort-Object { [int]$_.s }
$order = @{}
$actions | ForEach-Object { $order[$_.a] = [int]$_.s }
if ($order.StopRunningApp -ge $order.RemoveExistingProducts) { Fail "it closes the running copy too late" } else { Ok "closes what's running before removing the old version" }
foreach ($restore in "RestoreDesktop", "RestoreVideo", "RestoreMusic", "RestoreDisc") {
  if ($order[$restore] -le $order.FindRelatedProducts) { Fail "$restore runs before an upgrade is detected" }
}
if ($order.RemoveExistingProducts -ge $order.InstallFiles) { Fail "the new files would arrive before the old version goes" } else { Ok "the old version goes before the new one's files arrive" }

# ---- an administrative extract: files only, and nothing is registered
[void][Runtime.InteropServices.Marshal]::ReleaseComObject($db)
$target = Join-Path $Scratch "extract"
if (Test-Path $target) { Remove-Item -Recurse -Force $target }
New-Item -ItemType Directory -Force $target | Out-Null
$log = Join-Path $Scratch "extract.log"
$proc = Start-Process msiexec.exe -ArgumentList @("/a", "`"$Msi`"", "/qn", "TARGETDIR=`"$target`"", "/l*v", "`"$log`"") -Wait -PassThru
Write-Host "msiexec /a exit code: $($proc.ExitCode)"
if ($proc.ExitCode -ne 0) { Fail "the administrative extract failed: see $log" }
$root = Get-ChildItem -Path $target -Recurse -Filter WittyMediaPlayer.exe -File | Select-Object -First 1
if (-not $root) {
  Fail "WittyMediaPlayer.exe wasn't in the extract"
} else {
  $dir = $root.Directory.FullName
  Write-Host "extracted to: $dir"
  foreach ($f in "Uninstall WittyMediaPlayer.exe", "package-files.txt", "bin\wittymediaplayer-engine.exe", "bin\wittymediaplayer-updater.exe", "Qt6Core.dll", "avcodec-62.dll", "platforms\qwindows.dll", "LICENSE", "NOTICE", "README.md", "licenses\THIRD-PARTY.txt", "licenses\LGPL-3.0.txt", "licenses\GPL-3.0.txt") {
    if (-not (Test-Path -LiteralPath (Join-Path $dir $f))) { Fail "$f is missing" }
  }
  $listed = Get-Content -LiteralPath (Join-Path $dir "package-files.txt") | Select-Object -Skip 1
  $actual = Get-ChildItem -LiteralPath $dir -Recurse -File | Where-Object { $_.Extension -ne ".msi" } | ForEach-Object { $_.FullName.Substring($dir.Length + 1).Replace('\', '/') }
  $diff = Compare-Object ($listed | Sort-Object) ($actual | Sort-Object)
  if ($diff) { Fail "the list of files doesn't match what came out"; $diff | ForEach-Object { Write-Host "  $($_.SideIndicator) $($_.InputObject)" } } else { Ok "all $($actual.Count) files on the package's list came out, and nothing else" }
  if ($PackageDir) {
    $bad = 0
    foreach ($f in $listed) {
      if ((Get-FileHash -LiteralPath (Join-Path $dir $f)).Hash -ne (Get-FileHash -LiteralPath (Join-Path $PackageDir $f)).Hash) { Fail "$f differs from the package"; $bad++ }
    }
    if ($bad -eq 0) { Ok "every file is the same as in the portable package" }
  }

  # What Properties and Task Manager will say.
  foreach ($exe in "WittyMediaPlayer.exe", "Uninstall WittyMediaPlayer.exe", "bin\wittymediaplayer-engine.exe", "bin\wittymediaplayer-updater.exe") {
    $v = (Get-Item -LiteralPath (Join-Path $dir $exe)).VersionInfo
    Write-Host ("{0,-36} description=[{1}] product=[{2}] version=[{3}] file=[{4}] original=[{5}] copyright=[{6}]" -f $exe, $v.FileDescription, $v.ProductName, $v.ProductVersion, $v.FileVersion, $v.OriginalFilename, $v.LegalCopyright)
    if (-not $v.FileDescription -or $v.ProductName -ne "WittyMediaPlayer" -or -not $v.ProductVersion.StartsWith($Version) -or $v.OriginalFilename -ne (Split-Path $exe -Leaf)) { Fail "$exe's details are wrong" }
    Add-Type -AssemblyName System.Drawing
    $icon = [System.Drawing.Icon]::ExtractAssociatedIcon((Join-Path $dir $exe))
    if (-not $icon) { Fail "$exe has no icon" } else { $icon.Dispose() }
  }
  Ok "every program has an icon, a description and a version"

  # Nothing the install would need to register has been: it's still only files.
  foreach ($k in "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WittyMediaPlayer", "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WittyMediaPlayer") {
    if (Test-Path $k) { Fail "$k exists: this machine has it installed" }
  }
}

if ($failed) { Write-Host "FAILED: $failed"; exit 1 }
Write-Host "ALLPASSED"
