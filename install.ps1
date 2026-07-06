# Asobi CLI installer for Windows.
#
# Usage:
#   irm https://raw.githubusercontent.com/widgrensit/asobi-cli/main/install.ps1 | iex
#
# Environment variables:
#   ASOBI_VERSION       Install a specific tag (e.g. v0.1.0). Default: latest release.
#   ASOBI_INSTALL_DIR   Install directory. Default: $env:LOCALAPPDATA\asobi\bin.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = 'widgrensit/asobi-cli'
$installDir = if ($env:ASOBI_INSTALL_DIR) { $env:ASOBI_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'asobi\bin' }

function Fail($msg) {
	Write-Error $msg
	exit 1
}

function Detect-Arch {
	switch ($env:PROCESSOR_ARCHITECTURE) {
		'AMD64' { 'amd64' }
		'ARM64' { 'arm64' }
		'x86'   { Fail 'unsupported architecture: x86 (32-bit). asobi ships amd64 and arm64 builds only.' }
		default { Fail "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
	}
}

function Latest-Version {
	try {
		$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'asobi-install' }
	} catch {
		Fail "could not query the latest release: $($_.Exception.Message)"
	}
	$release.tag_name
}

function Verify-Checksum($file, $sumsFile, $name) {
	$line = Select-String -Path $sumsFile -Pattern "  $([regex]::Escape($name))$" | Select-Object -First 1
	if (-not $line) { Fail "no checksum found for $name in checksums.txt" }
	$expected = ($line.Line -split '\s+')[0]
	$actual = (Get-FileHash -Path $file -Algorithm SHA256).Hash.ToLower()
	if ($expected.ToLower() -ne $actual) {
		Fail "checksum mismatch for $name (expected $expected, got $actual)"
	}
}

function Add-ToUserPath($dir) {
	$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
	$entries = if ($userPath) { $userPath -split ';' } else { @() }
	if ($entries -notcontains $dir) {
		$newPath = if ($userPath) { "$userPath;$dir" } else { $dir }
		[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
		Write-Host ''
		Write-Host "Added $dir to your user PATH. Open a new terminal, then run 'asobi version'."
	} else {
		Write-Host "Run 'asobi version' to get started."
	}
}

$arch = Detect-Arch
$version = if ($env:ASOBI_VERSION) { $env:ASOBI_VERSION } else { Latest-Version }
if (-not $version) { Fail 'could not resolve a release version' }

$asset = "asobi_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/$version"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("asobi-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
	Write-Host "Downloading asobi $version (windows/$arch)..."
	try {
		Invoke-WebRequest -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset) -UseBasicParsing
	} catch {
		Fail "failed to download $asset - check that $version has a windows/$arch build"
	}
	Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

	Write-Host 'Verifying checksum...'
	Verify-Checksum (Join-Path $tmp $asset) (Join-Path $tmp 'checksums.txt') $asset

	Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force
	$binary = Join-Path $tmp 'asobi.exe'
	if (-not (Test-Path $binary)) { Fail "asobi.exe not found in $asset" }

	New-Item -ItemType Directory -Path $installDir -Force | Out-Null
	$installPath = Join-Path $installDir 'asobi.exe'
	Move-Item -Path $binary -Destination $installPath -Force

	Write-Host "Installed asobi to $installPath"
	Add-ToUserPath $installDir
} finally {
	Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
