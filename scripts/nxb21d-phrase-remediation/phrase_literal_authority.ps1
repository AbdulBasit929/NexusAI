# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest

$script:NxDPhraseQuotePattern = [regex]::new('"([^"]+)"|\u201C([^\u201D]+)\u201D|''([^'']+)''|\u2018([^\u2019]+)\u2019', [Text.RegularExpressions.RegexOptions]::CultureInvariant)
$script:NxDDocumentHintPattern = [regex]::new('(?i)\b(document|pdf|docx|passage)\b|\u062F\u0633\u062A\u0627\u0648\u06CC\u0632', [Text.RegularExpressions.RegexOptions]::CultureInvariant)
$script:NxDAudioHintPattern = [regex]::new('(?i)\b(transcript|transcription|recording|audio)\b|\u0679\u0631\u0627\u0646\u0633\u06A9\u0631\u067E\u0679|\u0631\u06CC\u06A9\u0627\u0631\u0688\u0646\u06AF', [Text.RegularExpressions.RegexOptions]::CultureInvariant)

function Get-NxDCodepoints([AllowEmptyString()][string]$Value) {
    $values = New-Object 'System.Collections.Generic.List[string]'
    for ($index = 0; $index -lt $Value.Length; $index++) {
        $codepoint = [char]::ConvertToUtf32($Value, $index)
        if ([char]::IsHighSurrogate($Value[$index])) { $index++ }
        $values.Add(('U+{0:X4}' -f $codepoint))
    }
    return @($values | ForEach-Object { $_ })
}

function Get-NxDNormalizedPhrase([AllowEmptyString()][string]$Value) {
    return ([regex]::Replace($Value.Normalize([Text.NormalizationForm]::FormC), '\s+', ' ')).Trim()
}

function Test-NxDPhraseScriptMismatch([string]$Source, [string]$Model) {
    $sourceArabic = [regex]::IsMatch($Source, '[\u0600-\u06FF]')
    $sourceCyrillic = [regex]::IsMatch($Source, '[\u0400-\u04FF]')
    $modelCyrillic = [regex]::IsMatch($Model, '[\u0400-\u04FF]')
    return [bool]($sourceArabic -and -not $sourceCyrillic -and $modelCyrillic)
}

function Resolve-NxDExplicitPhraseLiteral(
    [string]$Question,
    [string]$CapabilityID,
    [string]$Semantic,
    [AllowEmptyString()][string]$ModelLiteral,
    [bool]$ModelClarification
) {
    $wantedFamily = switch ($CapabilityID) {
        'document.phrase' { 'document' }
        'transcript.phrase' { 'audio' }
        'roman_urdu.phrase' { 'audio' }
        default { '' }
    }
    if (-not $wantedFamily -or @('EXACT_VALUE', 'EXACT_PHRASE', 'PHRASE_CONTAINS', 'TOKEN_SEARCH') -cnotcontains $Semantic) {
        return [pscustomobject][ordered]@{applicable = $false; resolution_status = 'NOT_APPLICABLE'}
    }
    $matches = @($script:NxDPhraseQuotePattern.Matches($Question))
    if ($matches.Count -eq 0) {
        return [pscustomobject][ordered]@{applicable = $false; resolution_status = 'NOT_APPLICABLE_NO_EXPLICIT_SPAN'}
    }
    if ($matches.Count -ne 1) {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'MULTIPLE_EXPLICIT_PHRASE_SPANS'; source_spans = @($matches | ForEach-Object { [string]$_.Value })}
    }
    $match = $matches[0]
    $source = ''
    for ($group = 1; $group -lt $match.Groups.Count; $group++) {
        if ($match.Groups[$group].Success) { $source = [string]$match.Groups[$group].Value; break }
    }
    $outside = $Question.Remove($match.Index, $match.Length)
    $families = New-Object 'System.Collections.Generic.List[string]'
    if ($script:NxDDocumentHintPattern.IsMatch($outside)) { $families.Add('document') }
    if ($script:NxDAudioHintPattern.IsMatch($outside)) { $families.Add('audio') }
    if ($families.Count -ne 1 -or $families[0] -cne $wantedFamily) {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'PHRASE_FAMILY_ABSENT_OR_CONTRADICTORY'; source_spans = @($source)}
    }
    $wantedSemantic = if ($outside.ToLowerInvariant().Contains('exact phrase')) { 'EXACT_PHRASE' } else { 'PHRASE_CONTAINS' }
    if ($Semantic -cne $wantedSemantic) {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'PHRASE_SEMANTIC_CONTRADICTION'; source_spans = @($source)}
    }
    $reasons = New-Object 'System.Collections.Generic.List[string]'
    if ($ModelLiteral -cne $source) { $reasons.Add('literal_not_exact_source_span') }
    if ($ModelClarification) { $reasons.Add('unjustified_model_clarification') }
    $scriptMismatch = Test-NxDPhraseScriptMismatch $source $ModelLiteral
    if ($scriptMismatch) { $reasons.Add('model_script_family_mismatch') }
    $mismatches = @($reasons | ForEach-Object { $_ })
    return [pscustomobject][ordered]@{
        contract_version = 'forensics.phrase-literal-authority/v1'
        applicable = $true
        capability_id = $CapabilityID
        semantic = $Semantic
        cardinality = 'exactly_one_explicit_quoted_span'
        source_literal = $source
        source_codepoints = @(Get-NxDCodepoints $source)
        source_normalized = Get-NxDNormalizedPhrase $source
        model_literal = $ModelLiteral
        model_codepoints = @(Get-NxDCodepoints $ModelLiteral)
        model_normalized = Get-NxDNormalizedPhrase $ModelLiteral
        model_clarification = $ModelClarification
        model_proposal_status = if ($mismatches.Count) { 'FAIL_PHRASE_LITERAL' } else { 'PASS' }
        mismatch_reasons = $mismatches
        script_family_mismatch = $scriptMismatch
        reconciled = [bool]($mismatches.Count)
        reconciliation_reason = 'one explicit quoted phrase and matching typed capability/scope context'
        resolution_status = 'RESOLVED_DETERMINISTIC_AUTHORITY'
        final_literal = $source
        final_codepoints = @(Get-NxDCodepoints $source)
        final_clarification = $false
        normalization_contract = 'nfc-whitespace/v1'
    }
}
