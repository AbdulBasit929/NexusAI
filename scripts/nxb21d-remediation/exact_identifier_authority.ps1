# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest

$script:NxDPlateSpanPattern = [regex]::new('(?i)\b[A-Z]{1,4}[- ]?\d{1,6}[A-Z]{0,3}\b', [Text.RegularExpressions.RegexOptions]::CultureInvariant)

function Get-NxDAuthoritativeExactIdentifierSpans(
    [string]$Question,
    [string]$CapabilityID
) {
    if ($CapabilityID -cne 'image.plate') { return @() }
    $values = New-Object 'System.Collections.Generic.List[string]'
    foreach ($match in $script:NxDPlateSpanPattern.Matches($Question)) {
        # Match.Value is the exact source substring; no normalization occurs.
        if (-not $values.Contains([string]$match.Value)) {
            $values.Add([string]$match.Value)
        }
    }
    return @($values | ForEach-Object { $_ })
}

function Resolve-NxDSingleExactIdentifier(
    [string]$Question,
    [string]$CapabilityID,
    [string]$Semantic,
    [AllowEmptyString()][string]$ModelLiteral,
    [AllowEmptyCollection()][string[]]$ModelEntities
) {
    if ($CapabilityID -cne 'image.plate' -or $Semantic -cne 'EXACT_VALUE') {
        return [pscustomobject][ordered]@{applicable = $false; resolution_status = 'NOT_APPLICABLE'}
    }
    $spans = @(Get-NxDAuthoritativeExactIdentifierSpans $Question $CapabilityID)
    if ($spans.Count -eq 0) {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'NO_BOUNDED_IDENTIFIER_SPAN'; authoritative_spans = @()}
    }
    if ($spans.Count -ne 1) {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'MULTIPLE_IDENTIFIER_SPANS_UNSUPPORTED'; authoritative_spans = $spans}
    }
    $authoritative = [string]$spans[0]
    $entities = @($ModelEntities | ForEach-Object { [string]$_ })
    $reasons = New-Object 'System.Collections.Generic.List[string]'
    if ($ModelLiteral -cne $authoritative) { $reasons.Add('literal_not_exact_source_span') }
    if ($entities.Count -ne 1) { $reasons.Add('entity_count_not_one') }
    if ($entities.Count -ne 1 -or $entities[0] -cne $authoritative) { $reasons.Add('atomic_entity_not_exact_source_span') }
    foreach ($entity in $entities) {
        if (-not $Question.Contains($entity)) { $reasons.Add('model_entity_not_in_question'); break }
    }
    $mismatches = @($reasons | ForEach-Object { $_ })
    return [pscustomobject][ordered]@{
        contract_version = 'forensics.exact-identifier-authority/v1'
        applicable = $true
        capability_id = $CapabilityID
        semantic = $Semantic
        cardinality = 'exactly_one'
        authoritative_spans = $spans
        model_literal_text = $ModelLiteral
        model_entities = $entities
        model_proposal_status = if ($mismatches.Count) { 'FAIL_ATOMIC_IDENTIFIER' } else { 'PASS' }
        mismatch_reasons = $mismatches
        reconciled = [bool]($mismatches.Count)
        resolution_status = 'RESOLVED_DETERMINISTIC_AUTHORITY'
        final_literal_text = $authoritative
        final_entities = @($authoritative)
    }
}
