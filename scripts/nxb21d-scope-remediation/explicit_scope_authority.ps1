# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest

$script:NxDScopeQuotePattern = [regex]::new('"([^"]+)"|\u201C([^\u201D]+)\u201D|''([^'']+)''|\u2018([^\u2019]+)\u2019', [Text.RegularExpressions.RegexOptions]::CultureInvariant)
$script:NxDWorkspaceScopePhrases = @(
    'whole investigation', 'across the investigation', 'across the case', 'all evidence', 'entire case',
    'all recordings', 'every recording', 'all transcripts', 'every transcript', 'all transcript files', 'every transcript file',
    'all documents', 'every document', 'all document files', 'every document file',
    'poori investigation', 'saray saboot', 'sare saboot', 'tamam evidence',
    'tamam recordings', 'tamam case recordings', 'tamam transcripts', 'tamam transcript files', 'tamam documents', 'tamam document files',
    '\u067E\u0648\u0631\u06D2 \u06A9\u06CC\u0633', '\u067E\u0648\u0631\u06CC \u062A\u0641\u062A\u06CC\u0634', '\u062A\u0645\u0627\u0645 \u0634\u0648\u0627\u06C1\u062F',
    '\u062A\u0645\u0627\u0645 \u0631\u06CC\u06A9\u0627\u0631\u0688\u0646\u06AF', '\u062A\u0645\u0627\u0645 \u0679\u0631\u0627\u0646\u0633\u06A9\u0631\u067E\u0679', '\u062A\u0645\u0627\u0645 \u062F\u0633\u062A\u0627\u0648\u06CC\u0632\u0627\u062A'
)
$script:NxDSelectedScopePhrases = @(
    'this recording', 'this audio', 'this image', 'this document', 'this video',
    'is recording', 'is audio', 'is tasveer', 'is document', 'is video',
    '\u0627\u0633 \u0631\u06CC\u06A9\u0627\u0631\u0688\u0646\u06AF', '\u0627\u0633 \u062A\u0635\u0648\u06CC\u0631', '\u0627\u0633 \u062F\u0633\u062A\u0627\u0648\u06CC\u0632', '\u0627\u0633 \u0648\u06CC\u0688\u06CC\u0648'
)

function ConvertFrom-NxDScopeEscapes([string]$Value) {
    return [regex]::Replace($Value, '\\u([0-9A-Fa-f]{4})', { param($m) [char][Convert]::ToInt32($m.Groups[1].Value, 16) })
}

function Get-NxDExplicitScope([string]$Question) {
    $outside = $script:NxDScopeQuotePattern.Replace($Question, ' ')
    $normalized = ([regex]::Replace($outside.ToLowerInvariant(), '\s+', ' ')).Trim()
    $workspace = $false
    $selected = $false
    foreach ($phrase in $script:NxDWorkspaceScopePhrases) {
        if ($normalized.Contains((ConvertFrom-NxDScopeEscapes $phrase))) { $workspace = $true; break }
    }
    foreach ($phrase in $script:NxDSelectedScopePhrases) {
        if ($normalized.Contains((ConvertFrom-NxDScopeEscapes $phrase))) { $selected = $true; break }
    }
    if ($workspace -and $selected) { return 'ambiguous' }
    if ($workspace) { return 'workspace' }
    if ($selected) { return 'selected' }
    return ''
}

function Resolve-NxDExplicitScopeAuthority([string]$Question, [AllowEmptyString()][string]$ModelScope) {
    $sourceScope = Get-NxDExplicitScope $Question
    if (-not $sourceScope) {
        return [pscustomobject][ordered]@{applicable = $false; resolution_status = 'NOT_APPLICABLE_NO_EXPLICIT_SCOPE'}
    }
    if ($sourceScope -ceq 'ambiguous') {
        return [pscustomobject][ordered]@{applicable = $true; resolution_status = 'REJECTED_REQUIRES_CLARIFICATION'; reason = 'CONTRADICTORY_EXPLICIT_SCOPE'}
    }
    $reconciled = $ModelScope -cne $sourceScope
    return [pscustomobject][ordered]@{
        contract_version = 'forensics.explicit-scope-authority/v1'
        applicable = $true
        source_scope = $sourceScope
        model_scope = $ModelScope
        model_proposal_status = if ($reconciled) { 'FAIL_SCOPE' } else { 'PASS' }
        reconciled = $reconciled
        reconciliation_reason = 'bounded explicit scope outside quoted evidence text'
        resolution_status = 'RESOLVED_DETERMINISTIC_AUTHORITY'
        final_scope = $sourceScope
    }
}
