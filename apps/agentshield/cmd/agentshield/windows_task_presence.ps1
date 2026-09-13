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
    try {
        $task = $folder.GetTask($request.task_name)
    } catch {
        $cause = $_.Exception
        while ($null -ne $cause.InnerException) { $cause = $cause.InnerException }
        if (($cause -is [System.Runtime.InteropServices.COMException]) -and ($cause.HResult -eq -2147024894)) {
            [Console]::Out.Write('SIQ_TASK_ABSENT')
            exit 0
        }
        throw
    }
    if (($null -eq $task) -or ($task.Path -cne $request.task_name)) { throw 'wrong task' }
    [Console]::Out.Write('SIQ_TASK_PRESENT')
    exit 0
} catch {
    [Console]::Error.Write('SIQ task query failed')
    exit 1
}
