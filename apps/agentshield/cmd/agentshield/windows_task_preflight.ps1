$ErrorActionPreference = 'Stop'
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if ($sid -cne $request.user_sid) { throw 'wrong user' }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $folder = $service.GetFolder('\')
    if (($null -eq $folder) -or ($folder.Path -cne '\')) { throw 'wrong folder' }
    [Console]::Out.Write('SIQ_TASK_MANAGER_READY')
    exit 0
} catch {
    [Console]::Error.Write('SIQ task manager preflight failed')
    exit 1
}
