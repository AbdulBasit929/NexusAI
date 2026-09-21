# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')

$script:NxSttManifestPath = Resolve-NxPath 'configuration/nxmmr_shared_query_activation_v1.json'
$script:NxSttIntegrityPath = Resolve-NxPath 'configuration/nxmmr_shared_query_operator_integrity_v1.json'
$script:NxSttChanged = @('api', 'forensic-records-api', 'forensic-records-worker')
$script:NxSttProtected = @('forensic-postgres', 'forensic-nats')
$script:NxSttRunRoot = Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation-shared-query'

function Get-NxSttManifest {
    $seal = Get-Content -LiteralPath $script:NxSttIntegrityPath -Raw | ConvertFrom-Json
    if ($seal.contract_version -ne 'nexusai.nxmmr.shared-query-operator-integrity/v1') { throw 'Shared-query integrity contract is invalid.' }
    foreach ($property in $seal.files.PSObject.Properties) {
        if ((Get-NxHash (Resolve-NxPath $property.Name)) -cne $property.Value) { throw "Source integrity mismatch: $($property.Name)" }
    }
    $manifest = Get-Content -LiteralPath $script:NxSttManifestPath -Raw | ConvertFrom-Json
    if ($manifest.contract_version -ne 'nexusai.nxmmr.shared-query-activation/v1') { throw 'Shared-query activation manifest is invalid.' }
    if (($manifest.services.build_and_recreate_no_deps -join ',') -cne ($script:NxSttChanged -join ',')) { throw 'Changed service scope differs from the admitted three-service boundary.' }
    if (($manifest.services.must_remain_identical -join ',') -cne ($script:NxSttProtected -join ',')) { throw 'Protected service scope changed.' }
    if ([double]$manifest.resource_gates.minimum_available_ram_gib -ne 6) { throw 'The 6 GiB RAM gate changed.' }
    $roles = ConvertTo-NxMap $manifest.worker_environment
    foreach ($name in @('FORENSIC_ASR_ENABLED','FORENSIC_IMAGE_EMBEDDING_ENABLED','FORENSIC_FACE_ENABLED','FORENSIC_ANPR_ENABLED','FORENSIC_OCR_ENABLED','FORENSIC_VIDEO_ANPR_V3_ENABLED')) {
        if ($roles[$name] -cne 'true') { throw "Required worker role is not enabled: $name" }
    }
    foreach ($name in @('FORENSIC_VIDEO_ANPR_V2_ENABLED','FORENSIC_TTS_ENABLED')) {
        if ($roles[$name] -cne 'false') { throw "Forbidden worker role is enabled: $name" }
    }
    if ($roles['FORENSIC_ASR_MODEL'] -cne 'faster-whisper-small-ur' -or $roles['HF_HUB_OFFLINE'] -cne '1' -or $roles['TRANSFORMERS_OFFLINE'] -cne '1') { throw 'ASR or offline model policy changed.' }
    return $manifest
}

function Find-NxSharedQueryResumableBuildSet($Before, [string]$CurrentRun) {
    $manifestHash=Get-NxHash $script:NxSttManifestPath
    $sealHash=Get-NxHash $script:NxSttIntegrityPath
    if(-not (Test-Path -LiteralPath $script:NxSttRunRoot)){return [ordered]@{}}
    foreach($directory in @(Get-ChildItem -LiteralPath $script:NxSttRunRoot -Directory | Sort-Object Name -Descending)){
        if($directory.FullName -eq $CurrentRun){continue}
        $path=Join-Path $directory.FullName 'build-set.json'
        if(-not (Test-Path -LiteralPath $path -PathType Leaf)){continue}
        try {
            $candidate=Get-Content -LiteralPath $path -Raw|ConvertFrom-Json
            if($candidate.contract_version -ne 'nexusai.nxmmr.shared-query-build-set/v1' -or $candidate.manifest_sha256 -cne $manifestHash -or $candidate.source_seal_sha256 -cne $sealHash -or $candidate.recreation_started){continue}
            $original=ConvertTo-NxMap $candidate.original_containers;$same=$true
            foreach($name in $script:NxSttChanged){if($original[$name] -cne $Before[$name].Id){$same=$false;break}}
            if(-not $same){continue}
            $valid=[ordered]@{}
            foreach($property in $candidate.images.PSObject.Properties){
                if($property.Name -notin $script:NxSttChanged){continue}
                $actual=Invoke-NxNative docker @('image','inspect',[string]$property.Value.tag,'--format','{{.Id}}')
                if($actual -cne [string]$property.Value.id){throw "Candidate image tag drifted: $($property.Name)"}
                $valid[$property.Name]=[ordered]@{tag=[string]$property.Value.tag;id=[string]$property.Value.id}
            }
            if($valid.Count){Write-Host "RESUMABLE_BUILD_SET=PASS receipt=$path images=$($valid.Keys -join ',')";return $valid}
        } catch {Write-Host "RESUMABLE_BUILD_SET=REJECTED receipt=$path reason=$($_.Exception.Message)"}
    }
    return [ordered]@{}
}
