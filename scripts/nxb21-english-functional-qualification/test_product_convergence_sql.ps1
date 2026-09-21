[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$name='nxb21_group_v2_'+[guid]::NewGuid().ToString('N').Substring(0,16)
$password=[guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')
$createdRole=$false;$createdDB=$false;$passed=$false;$cleaned=$false
$report=Join-Path $root ('reports\nxb21\convergence-sql-api-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')+'.json')
function SQL([string]$statement) {
 $out=$statement | & docker exec -i nexusai-forensic-postgres-1 psql -X -v ON_ERROR_STOP=1 -U localrecall -d localrecall -At
 if($LASTEXITCODE -ne 0){throw 'DISPOSABLE_SQL_COMMAND_FAILED'}
 return ($out -join "`n").Trim()
}
$retainedSQL="SELECT (SELECT count(*) FROM forensic.evidence_items)||'|'||(SELECT count(*) FROM forensic.evidence_versions)||'|'||(SELECT count(*) FROM forensic.records_ingest_jobs)||'|'||(SELECT count(*) FROM forensic.records)||'|'||(SELECT count(*) FROM forensic.derived_artifacts)||'|'||(SELECT count(*) FROM forensic.kb_collection_assets);"
$before=SQL $retainedSQL
try {
 if($name -cnotmatch '^nxb21_group_v2_[a-f0-9]{16}$'){throw 'UNSAFE_SCRATCH_ID'}
 if((SQL "SELECT count(*) FROM pg_database WHERE datname='$name';") -ne '0'){throw 'DATABASE_ALREADY_EXISTS'}
 if((SQL "SELECT count(*) FROM pg_roles WHERE rolname='$name';") -ne '0'){throw 'ROLE_ALREADY_EXISTS'}
 $null=SQL "CREATE ROLE $name LOGIN PASSWORD '$password' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;";$createdRole=$true
 $null=SQL "CREATE DATABASE $name OWNER $name;";$createdDB=$true
 $env:NXB21_GROUP_TEST_DATABASE_NAME=$name
 $env:NXB21_GROUP_TEST_DATABASE_URL="postgres://${name}:${password}@127.0.0.1:5433/${name}?sslmode=disable"
 Push-Location $root
 try {
  & go test ./api/forensic_records -count=1 -run '^TestForensicRecordsSynthesis$' '-ginkgo.succinct' '-ginkgo.focus=Disposable canonical group SQL API|English V2 real discovery' '-ginkgo.no-color' 2>&1 | Tee-Object -FilePath ($report+'.log')
  if($LASTEXITCODE -ne 0){throw 'GROUP_ACCEPTANCE_FAILED'}
  $passed=$true
 } finally {Pop-Location}
} finally {
 $env:NXB21_GROUP_TEST_DATABASE_URL=$null;$env:NXB21_GROUP_TEST_DATABASE_NAME=$null
 if($createdDB){
  if((SQL "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$name';") -cne $name){throw 'SCRATCH_OWNERSHIP_CHANGED'}
  $null=SQL "DROP DATABASE $name;"
 }
 if($createdRole){$null=SQL "DROP ROLE $name;"}
 $cleaned=$true
 $after=SQL $retainedSQL
 [ordered]@{contract='nexusai.convergence-sql-acceptance/v1';database=$name;passed=$passed;cleanup=$cleaned;retained_before=$before;retained_after=$after;retained_unchanged=($before -ceq $after);live_inference=$false;null_fixture='source_file temporarily nullable only in owned scratch database';log=($report+'.log')} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $report -Encoding UTF8
 Write-Host "GROUP_ACCEPTANCE_RECEIPT=$report"
 if($before -cne $after){throw 'RETAINED_STATE_DRIFT'}
}
