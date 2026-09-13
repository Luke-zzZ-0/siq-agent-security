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
    $before = [int]$task.State
    $count = [int]$task.GetInstances(0).Count
    $last = [long]$task.LastTaskResult
    if ([int]$task.State -ne $before) { throw 'state changed' }
    $culture = [System.Globalization.CultureInfo]::InvariantCulture
    [Console]::Out.Write('SIQ_TASK_RUNTIME:' + $before.ToString($culture) + ':' + $count.ToString($culture) + ':' + $last.ToString($culture))
    exit 0
} catch {
    [Console]::Error.Write('SIQ task runtime query failed')
    exit 1
}
