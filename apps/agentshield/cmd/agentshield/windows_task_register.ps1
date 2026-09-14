$ErrorActionPreference = 'Stop'
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    if ($request.task_name -cnotmatch '^\\SIQ-Agent-Security-[a-f0-9]{64}$') { throw 'invalid task' }
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if ($sid -cne $request.user_sid) { throw 'wrong user' }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $folder = $service.GetFolder('\')
    # Expand schema defaults before registration; the scheduling engine is
    # explicitly bound by the source. This does not register or run a task.
    $definition = $service.NewTask(0)
    $definition.XmlText = $request.task_xml
    # TASK_CREATE only: a competing existing task must never be updated.
    # TASK_LOGON_INTERACTIVE_TOKEN uses the already logged-in current user.
    $task = $folder.RegisterTask($request.task_name, $definition.XmlText, 2, $sid, $null, 3, $null)
    if (($null -eq $task) -or ($task.Path -cne $request.task_name)) { throw 'wrong task' }
    [Console]::Out.Write('SIQ_TASK_CREATED')
    exit 0
} catch {
    [Console]::Error.Write('SIQ task registration failed')
    exit 1
}
