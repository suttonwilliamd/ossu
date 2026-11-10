param([switch]$Test)
$ErrorActionPreference='Stop'
$root=$PSScriptRoot
$out=Join-Path $root 'build'
New-Item -ItemType Directory -Force $out | Out-Null
$csc=Join-Path $env:WINDIR 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
& $csc /nologo /target:winexe /optimize+ /platform:anycpu /r:System.Drawing.dll /r:System.Windows.Forms.dll /r:System.Xml.dll /r:System.Core.dll /out:"$out\OSSU.exe" "$root\src\OSSU.cs"
if($LASTEXITCODE -ne 0){throw 'Build failed'}
if($Test){$p=Start-Process -FilePath "$out\OSSU.exe" -ArgumentList @('--self-test',"`"$out\test-evidence`"") -WindowStyle Hidden -Wait -PassThru;if($p.ExitCode -ne 0){throw 'Tests failed'};Get-Content "$out\test-evidence\test-results.txt"}
