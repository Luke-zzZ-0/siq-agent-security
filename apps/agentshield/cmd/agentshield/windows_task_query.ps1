$ErrorActionPreference = 'Stop'
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    if ($request.task_name -cnotmatch '^\\SIQ-Agent-Security-[a-f0-9]{64}$') { throw 'invalid task' }
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if ($sid -cne $request.user_sid) { throw 'wrong user' }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $task = $service.GetFolder('\').GetTask($request.task_name)
    if (($null -eq $task) -or ($task.Path -cne $request.task_name)) { throw 'wrong task' }
    # Xml may omit schema defaults. Definition.XmlText expands the complete
    # actual configuration without translating through schtasks' OEM stdout.
    $xml = $task.Definition.XmlText
    $declaration = '<?xml version="1.0" encoding="UTF-16"?>'
    if (($xml.Length -gt 65536) -or (-not $xml.StartsWith($declaration))) { throw 'unsupported XML' }
    [Console]::Out.Write($xml.Substring($declaration.Length))
    exit 0
} catch {
    [Console]::Error.Write('SIQ task readback failed')
    exit 1
}
