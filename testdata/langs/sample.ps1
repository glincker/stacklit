using namespace System.Collections
Import-Module Az.Accounts
. ./helpers.ps1

function Get-Thing {
    param([string]$Name, [int]$Count)
    "x"
}

function Private-Thing { 1 }

class Widget {
    [string] $Name
    [void] Run() { }
}
