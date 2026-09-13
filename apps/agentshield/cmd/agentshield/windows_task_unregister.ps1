$ErrorActionPreference = 'Stop'
function Read-TaskXML([string] $text) {
    if (($null -eq $text) -or ($text.Length -gt 65536)) { throw 'invalid XML size' }
    $settings = [System.Xml.XmlReaderSettings]::new()
    $settings.DtdProcessing = [System.Xml.DtdProcessing]::Prohibit
    $settings.XmlResolver = $null
    $settings.MaxCharactersInDocument = 65536
    $inputText = [System.IO.StringReader]::new($text)
    $reader = [System.Xml.XmlReader]::Create($inputText, $settings)
    try {
        $document = [System.Xml.XmlDocument]::new()
        $document.XmlResolver = $null
        $document.PreserveWhitespace = $false
        $document.Load($reader)
        return $document.DocumentElement.OuterXml
    } finally {
        $reader.Dispose()
        $inputText.Dispose()
    }
}
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    if ($request.task_name -cnotmatch '^\\SIQ-Agent-Security-[a-f0-9]{64}$') { throw 'invalid task' }
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if ($sid -cne $request.user_sid) { throw 'wrong user' }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $folder = $service.GetFolder('\')
    $task = $folder.GetTask($request.task_name)
    if (($null -eq $task) -or ($task.Path -cne $request.task_name)) { throw 'wrong task' }
    $expected = Read-TaskXML $request.task_xml
    $actual = Read-TaskXML $task.Xml
    if ($actual -cne $expected) { throw 'task changed' }
    if (($task.State -ne 3) -or ($task.GetInstances(0).Count -ne 0) -or ($task.State -ne 3)) { throw 'task not idle' }
    $folder.DeleteTask($request.task_name, 0)
    [Console]::Out.Write('SIQ_TASK_DELETED')
    exit 0
} catch {
    [Console]::Error.Write('SIQ task deletion failed')
    exit 1
}
