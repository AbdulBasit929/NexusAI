# Forensic Content-Addressed Retention

Status: source and disposable-database acceptance complete on 2026-07-29;
deployment, migration, backup, legal-retention policy and infrastructure WORM
remain separately gated.

## Purpose

Every upload must be retained exactly once before classification or queueing.
The retained byte stream, not a multipart filename or declared size, is the
source of truth. This slice replaces mutable job-named spool files with scoped,
content-addressed objects and a separately protected receipt.

## Addressing contract

The layout version is `sha256-scope-v1`.

```text
forensic-spool://sha256-scope-v1/<scope-key>/<content-sha256>
forensic-spool-receipt://sha256-scope-v1/<scope-key>/<content-sha256>

<root>/objects/<scope-key>/<first-two-hash-characters>/<content-sha256>
<root>/receipts/<scope-key>/<first-two-hash-characters>/<content-sha256>.json
```

`content-sha256` is computed while streaming the actual upload. `scope-key` is
SHA-256 over UTF-8 length-prefixed tenant and collection values. Length prefixes
make the tuple unambiguous, while hashed path components prevent caller-provided
tenant, collection and filename values from shaping a filesystem path.

The original source filename remains metadata and is used for parser format
selection; it is never used as the physical object name.

## Write and verification lifecycle

1. Stream to a random incoming file under the same storage root while counting
   bytes and computing SHA-256.
2. Flush and synchronize the incoming file.
3. Publish with an atomic, create-only hard link. The destination is never
   opened for overwrite or truncate.
4. Mark the retained object read-only and synchronize its directory.
5. Perform a full read of the published object and verify regular-file type,
   exact byte count, SHA-256 and non-writable mode.
6. Create a bounded JSON receipt by the same create-only rule. It records the
   exact scope, hash, size, storage URI, original filename and retention time.
7. Return the stable object and receipt URIs to evidence/job metadata only after
   both verify.

Concurrent identical uploads converge on one object. The winner creates it;
other writers verify and reuse it. A receipt retains the first successful
retention timestamp.

If an existing hash address or receipt does not match, the service never repairs
it in place. The new incoming bytes are made read-only under
`integrity-conflicts/`, the upload fails closed, and only server logs receive the
quarantine path.

## Worker trust boundary

The worker receives the spool volume read-only. Before parsing a new-layout job,
it independently verifies:

- the resolved file remains below `FORENSIC_SPOOL_DIR` and is not a symlink;
- URI, scope key, content hash and canonical physical path agree;
- the object and receipt are non-writable regular files;
- the receipt has the exact bounded field set and agrees with job scope/size;
- a full read reproduces the declared byte count and SHA-256.

Jobs created before this layout remain readable for migration compatibility,
but even legacy paths must remain under the configured root and pass a full hash
check. Extensionless content objects are parsed using the preserved source
filename, not their physical path.

## Database representation

Migration 008 recognizes valid `sha256-scope-v1` metadata as backend
`forensic_spool_content_addressed`, sets `immutability_state=verified`,
`write_once=true`, and records verification time/method. Preflight recomputes the
scope key in PostgreSQL using the same length-prefixed UTF-8 contract and rejects
incomplete or inconsistent metadata. Legacy rows remain `legacy_spool` and are
not falsely promoted.

## Security and operational limits

This is a local application-level write-once contract, not a claim of regulatory
WORM or external notarization. A host/storage administrator or the API's storage
identity can still change filesystem state. Production activation therefore
requires reviewed ownership and ACLs, encrypted storage and transport, backups,
monitoring, legal-hold/retention/deletion policy, recovery testing and—where the
case standard requires it—an object store with independently enforced object
lock/WORM controls.

The source configuration has not been built or deployed, migration 008 has not
been applied to the retained registry, and no existing evidence bytes were
moved. Those actions require their own approvals and verified rollback points.

