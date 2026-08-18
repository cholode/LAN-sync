# perf6 连接容量阶梯压测驱动
# 逐级跑 k6 只建连场景：100 -> 500 -> 1000 -> 2000 -> 5000
# 任一级失败率 > 5% 即停止，不再跑更高一级
# 每级同步采样服务端 /metrics，结束后聚合
param(
  [string]$WsHost = "ws://8.130.151.211",
  [string]$MetricsUrl = "http://8.130.151.211:6060/metrics"
)

# PS 5.1 下原生命令写 stderr 会生成错误记录，Stop 会误杀脚本，故用 Continue
$ErrorActionPreference = "Continue"
$PSNativeCommandUseErrorActionPreference = $false # PS7+ 生效；5.1 忽略
$dir = $PSScriptRoot
Set-Location $dir

# 清理上一轮残留，避免聚合污染
Get-ChildItem -Path $dir -Filter "summary_*.json" -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem -Path $dir -Filter "k6_level_*.log" -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem -Path $dir -Filter "server_level_*.txt" -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem -Path (Join-Path $dir "metrics") -Filter "level_*.txt" -ErrorAction SilentlyContinue | Remove-Item -Force

$levels = @(
  @{ vu = 100;  ramp = 20;  hold = 20 },
  @{ vu = 500;  ramp = 30;  hold = 30 },
  @{ vu = 1000; ramp = 30;  hold = 30 },
  @{ vu = 2000; ramp = 60;  hold = 30 },
  @{ vu = 5000; ramp = 120; hold = 30 }
)

$results = @()
$stopAt = $null

foreach ($lv in $levels) {
  $vu   = $lv.vu
  $ramp = $lv.ramp
  $hold = $lv.hold
  $dur  = $ramp + $hold + 15
  $prefix = "level_${vu}"

  Write-Host "`n===== LEVEL $vu VU (ramp=${ramp}s hold=${hold}s) ====="

  # 1) 启动服务端指标采样
  $env:PERF_METRICS_URL = $MetricsUrl
  $sampler = Start-Process -FilePath "node" `
    -ArgumentList @("collect_metrics.mjs", $prefix, "$dur", "2") `
    -WorkingDirectory $dir -WindowStyle Hidden -PassThru
  Start-Sleep -Seconds 1

  # 2) 跑 k6（stdout/stderr 重定向到文件，避免 PowerShell 错误管道误杀）
  $env:PERF_TARGET_VUS   = "$vu"
  $env:PERF_RAMP_SECONDS = "$ramp"
  $env:PERF_HOLD_SECONDS = "$hold"
  $env:PERF_WS_HOST      = $WsHost
  $env:PERF_WS_PATH      = "/api/v1/ws"
  k6 run --summary-export "summary_${vu}.json" k6-ws-connect-hold.js `
    1> "k6_level_${vu}.log" 2> "k6_level_${vu}.err.log"
  $k6exit = $LASTEXITCODE
  Write-Host "k6 exit=$k6exit"

  # 3) 等采样器结束
  if ($sampler -and -not $sampler.HasExited) {
    Wait-Process -Id $sampler.Id -Timeout 30 -ErrorAction SilentlyContinue
    if (-not $sampler.HasExited) { Stop-Process -Id $sampler.Id -Force -ErrorAction SilentlyContinue }
  }

  # 4) 解析 k6 摘要
  $passes = 0; $fails = 0; $sessions = 0
  $summaryFile = "summary_${vu}.json"
  if (Test-Path $summaryFile) {
    $s = Get-Content $summaryFile -Raw | ConvertFrom-Json
    $passes   = [int64]$s.metrics.checks.passes
    $fails    = [int64]$s.metrics.checks.fails
    $sessions = [int64]$s.metrics.ws_sessions.count
  } else {
    Write-Host "[warn] summary file missing for level $vu"
  }
  $total = $passes + $fails
  $failRate = if ($total -gt 0) { [math]::Round($fails / $total * 100, 2) } else { 100.0 }

  # 5) 聚合服务端指标
  node analyze_metrics.mjs $prefix 1> "server_level_${vu}.txt" 2> "server_level_${vu}.err.txt"

  $results += [pscustomobject]@{
    Level     = $vu
    Sessions  = $sessions
    Passes    = $passes
    Fails     = $fails
    FailRatePct = $failRate
    K6Exit    = $k6exit
  }
  Write-Host "LEVEL $vu => sessions=$sessions passes=$passes fails=$fails failRate=${failRate}% k6exit=$k6exit"

  # 6) 判定是否继续
  if ($failRate -gt 5.0 -or $k6exit -ne 0) {
    $stopAt = $vu
    Write-Host ">> STOP: failure rate > 5% or k6 error at ${vu} VU"
    break
  }
}

# 7) 写初步报告
$rows = $results | ForEach-Object {
  "| $($_.Level) | $($_.Sessions) | $($_.Passes) | $($_.Fails) | $($_.FailRatePct)% | $($_.K6Exit) |"
}
$stopText = if ($null -eq $stopAt) { '全部通过' } else { "停止于 $stopAt 连接数" }
$report = @"
# perf6 连接容量阶梯压测报告（初步）

- 时间：$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
- 目标：$WsHost/api/v1/ws（仅建连，不发消息）
- 终止条件：失败率 > 5%

## 各级结果

| 目标连接数 | k6 sessions | 成功 | 失败 | 失败率 | k6 exit |
| --- | --- | --- | --- | --- | --- |
$($rows -join "`n")

- 停止级别：$stopText
"@
Set-Content -Path "压测报告.md" -Value $report -Encoding UTF8
Write-Host "`nDONE. stopAt=$stopAt  results written to 压测报告.md"
