$ErrorActionPreference = "Stop"
$now=get-date -Format("yyyyMMdd-HHmmss")
$outputPath = join-path $env:TEMP "container-logs-$now"
$containerdState = "c:\programdata\containerd\state"
mkdir $outputPath | Out-Null
$ErrorActionPreference = "SilentlyContinue"
Write-Host "- Gathering stack dumps, event logs, computer info and more:"

function gethveventlog($elName) {
    $ErrorActionPreference = "SilentlyContinue"
    Write-Host -NoNewLine "."
    $out=join-path $outputPath "$elName.evtx"
    if (Test-Path $out) { Remove-Item $out }
    wevtutil.exe epl $elName $out 2>&1 | Out-Null
}


function getnteventlog($elName) {
    $ErrorActionPreference = "SilentlyContinue"
    Write-Host -NoNewLine "."
    $log = Get-WmiObject -Class Win32_NTEventlogFile | Where-Object LogfileName -EQ "$elName" # | Out-Null
    $outPath = join-path $outputPath "$elName.evtx"
    if ($log -ne $null) {
        $log.BackupEventlog($outPath) | Out-Null
    }
}

$proc = (get-process containerd)
if ($proc -ne $null) {
    docker-signal.exe -pid $proc.Id  2>&1 | Out-Null
    $lookingFor = Join-Path $env:TEMP containerd.$($proc.Id).stacks.log
    if (Test-Path $lookingFor) {
        Copy-Item $lookingFor $outputPath
    }
}

# Gather a copy of the containerd state directory
# TODO: A means is needed to get from containerd what the state directory
# is so that we could for example use ctr.exe to extract it. Unfortunately
# ctr version API only has version and revision.
$state = Join-Path $outputPath "state"
mkdir $state | Out-Null
xcopy /C/H/R/S/Y $containerdState $state | Out-Null


$procs = (get-process containerd-shim-runhcs-v1)
if ($procs.Length -gt 0) {
    $procs | ForEach-Object {
        docker-signal.exe -pid $($_.Id) | Out-Null
        $lookingFor = Join-Path $env:TEMP containerd-shim-runhcs-v1.$($_.Id).stacks.log
        if (Test-Path $lookingFor) {
            Copy-Item $lookingFor $outputPath
        }
    }
}

$proc = (get-process dockerd)
if ($proc -ne $null) {
    docker-signal.exe -pid $proc.Id 2>&1 | Out-Null
    $drd=$(docker info -f "{{.DockerRootDir}}")
    $lookingFor = get-childitem $drd -Filter goroutine* | sort creationtime | select -expand fullname -last 1
    if (Test-Path $lookingFor) {
        Copy-Item $lookingFor $outputPath
    }
} 

# Get the process list
Write-Host -NoNewline "."
tasklist.exe | Out-File $(Join-Path $outputPath tasklist.txt)

# Save system an application event logs
getnteventlog "System"
getnteventlog "Application"

# Save all the Hyper-V event logs
$el = $(wevtutil.exe el)
$el | ForEach-Object {
    if ($_.StartsWith("Microsoft-Windows-Hyper")) {
        gethveventlog  $_
    }
}

# Save the drive info (includes drive letter, free and size)
$di = $(Join-Path $outputPath "driveinfo.txt")
Get-WmiObject -Class Win32_logicaldisk -Filter "DriveType = '3'" | Out-File $di

# Save the Operating system Info
$os = $(Join-Path $outputPath "win32_operatingsystem.txt")
Get-WmiObject -Class Win32_OperatingSystem | Out-File $os

# Save the ComputerInfo
$ci = $(Join-Path $outputPath "Get-ComputerInfo.txt")
Get-ComputerInfo | Out-File $ci



$zip = "c:\container-logs-$now.zip"
Write-Host ""
Write-Host "- Compressing"
Compress-Archive $outputPath/* -DestinationPath $zip
Remove-Item $outputPath -Recurse -Force
Write-Host "- Saved to $zip"
