$ErrorActionPreference = 'Stop'
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    if ($request.task_name -cnotmatch '^\\SIQ-Agent-Security-[a-f0-9]{64}$') { throw 'invalid task' }
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if ($sid -cne $request.user_sid) { throw 'wrong user' }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $task = $service.GetFolder('\').GetTask($request.task_name)
    if (($null -eq $task) -or ($task.Path -cne $request.task_name)) { throw 'wrong task' }
    $running = $task.Run($null)
    if ($null -eq $running) { throw 'start unconfirmed' }
    [Console]::Out.Write('SIQ_TASK_RUN_REQUESTED')
    exit 0
} catch {
    [Console]::Error.Write('SIQ task start failed')
    exit 1
}
