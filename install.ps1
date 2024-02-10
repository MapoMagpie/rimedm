#!/usr/bin/env pwsh
# inherit from https://deno.land/x/install@v0.1.4/install.ps1
# Copyright 2018 the Deno authors. All rights reserved. MIT license.

# required:
# 1. $repo or $r
# 2. $version or $v
# 2. $exe or $e

$ErrorActionPreference = 'Stop'

$githubUrl = if ($github) {
  "${github}"
} elseif ($g) {
  "${g}"
}else {
  "https://github.com"
}

$owner = "MapoMagpie"
$repoName = "rimedm"
$exeName = "rimedm"

if ([Environment]::Is64BitProcess) {
  $arch = "x86_64"
} else {
  $arch = "i386"
}

$BinDir = "$Home\Appdata\Local\Programs\rimedm"
$downloadedTagGz = "$BinDir\${exeName}.zip"
$downloadedExe = "$BinDir\${exeName}.exe"
$Target = "Windows_$arch"

# GitHub requires TLS 1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$ResourceUri = if (!$version) {
  "${githubUrl}/${owner}/${repoName}/releases/latest/download/${exeName}_${Target}.zip"
} else {
  "${githubUrl}/${owner}/${repoName}/releases/download/${Version}/${exeName}_${Target}.zip"
}

if (!(Test-Path $BinDir)) {
  New-Item $BinDir -ItemType Directory | Out-Null
}

Invoke-WebRequest $ResourceUri -OutFile $downloadedTagGz -UseBasicParsing -ErrorAction Stop

function Check-Command {
  param($Command)
  $found = $false
  try
  {
      $Command | Out-Null
      $found = $true
  }
  catch [System.Management.Automation.CommandNotFoundException]
  {
      $found = $false
anakan
}

  $found
}

function Unzip($zipFile, $dest) {
  Expand-Archive -Force -Path $zipFile -DestinationPath $dest
}

Unzip $downloadedTagGz $BinDir

Remove-Item $downloadedTagGz

$User = [EnvironmentVariableTarget]::User
$Path = [Environment]::GetEnvironmentVariable('Path', $User)
if (!(";$Path;".ToLower() -like "*;$BinDir;*".ToLower())) {
  [Environment]::SetEnvironmentVariable('Path', "$Path;$BinDir", $User)
  $Env:Path += ";$BinDir"
}

Write-Output "${exeName} was installed successfully to $downloadedExe"
Write-Output "Run '${exeName} --h' to get started"
