Below is a **clean, authoritative rewrite** of the document, tightened into “source-of-truth” language. This is the version you can safely reference from Jira tickets, ADRs, and future domain work without ambiguity.

---

# Media Attachment Lifecycle & Attachment / Deletion Rules

**Authoritative Specification**

## Purpose

This document defines the **canonical lifecycle rules** for `media_attachments`.
It is the **single source of truth** for how media is linked to domain entities, how those links are created and removed, and under what conditions media may be deleted.

All domains that reference media (**products, ads, stores, users, licenses, banners, avatars, etc.**) MUST comply with the rules defined here.

---

## Core Model

A **Media** row represents a physical object in storage (e.g. GCS).
A **Media Attachment** row represents a *single, concrete usage* of that media by a domain entity.

### Invariants

* **One attachment row per usage**

  * Each reference from an entity to a media item MUST be represented by exactly one `media_attachments` row.
  * Attachments are never shared or reused across entities.

* **Attachment rows are immutable**

  * `media_id`, `entity_type`, `entity_id`, `store_id`, and `gcs_key` MUST NOT be mutated after creation.
  * To change usage, the attachment row is deleted and a new one is created.

* **Store scoping is mandatory**

  * Every attachment MUST include `store_id`, copied from the owning entity.
  * Cross-tenant attachment creation is invalid and MUST be rejected.

---

## Canonical Attachment Linking Rules (Creation & Removal)

### Attachment creation (MANDATORY)

A `media_attachments` row MUST be created **every time** a media item becomes referenced by an entity, including but not limited to:

* Product gallery images
* Product COA documents
* Ad creatives
* Store logos and banners
* User avatars
* License documents

Attachment creation MUST include:

* `media_id`
* `entity_type`
* `entity_id`
* `store_id`
* `gcs_key`
* `created_at`
* `is_protected` (derived from `entity_type` at creation time)

---

### Attachment removal (MANDATORY)

A `media_attachments` row MUST be removed when the entity **no longer references** that media item, including:

* Media removed from a gallery list
* Media replaced in a single-media field (logo, banner, avatar, COA, etc.)
* Media explicitly cleared by domain logic

Attachment removal MUST be driven by **explicit domain intent**.
No background or implicit cleanup is permitted.

---

### Update reconciliation rule (CRITICAL)

Any domain **create or update** operation that modifies media references MUST reconcile attachments by diffing:

* **Previously persisted media IDs**
* **New media IDs from the request**

Reconciliation rules:

* Media ID removed → delete the corresponding attachment row
* Media ID added → create a new attachment row
* Media ID unchanged → no-op

This applies to:

* **Single-media fields** (avatar, logo, banner, COA)
* **Multi-media collections** (product galleries, ad creatives)

---

### Transactionality requirement

Attachment creation and removal MUST occur **inside the same database transaction** as the domain mutation that changes the media reference.

If the domain transaction rolls back:

* Attachment rows MUST also roll back.

This is non-negotiable and required to prevent orphaned or phantom attachments.

### Reconciliation helper

The canonical helper `internal/media.NewAttachmentReconciler` enforces these rules. It accepts `entity_type`, `entity_id`, `store_id`, `old_media_ids`, and `new_media_ids`, runs inside the caller’s transaction, diffs the sets, creates new attachments for added IDs, and deletes rows for removed IDs while ensuring each attachment uses the tenant’s `store_id` and cached `gcs_key`. Every domain that touches media references MUST call this helper instead of writing raw `media_attachments` statements.

---

## Protected vs Unprotected Attachments

### Protected attachments

Attachments with the following `entity_type` values are **always protected**:

* `license`
* `ad`

If a media item has **any protected attachment**, the media **MUST NOT be deleted**.

Protection rules are defined in code via `ProtectedAttachmentEntities`
(e.g. `pkg/db/models/media_attachment.go`) and MUST remain in sync with this document.

---

### Unprotected attachments

All other entity types (products, store assets, user avatars, etc.) are unprotected.

Unprotected attachments MAY be removed **only** when:

* No protected attachments remain for that media.

Then emit a message to recursively delete the attachmet from the respected entities + the rows themselves aftwerwards. 

---

## Media Deletion Rules

### Deletion preconditions (STRICT)

Before deleting a Media row, the service MUST:

1. **Load all attachments** referencing the media
2. **Reject deletion** if any attachment is protected -> return to client
**non-protected attachments**
4. **Delete the Media row**

   * Either trigger:

     * GCS object deletion event, or
     * outbox event emitted in the delete transaction
5. **Detach non-protected attachments**

   * Domain-specific cleanup MUST run first (gallery updates, banner resets, avatar clears, etc.)
6. **Delete attachment rows**

Deletion MUST NOT skip or reorder these steps.

---

## Enforcement Model

* Enforcement lives in the **service layer**, not the database.
* Domain services MUST:

  * Consult `ProtectedAttachmentEntities`
  * Reconcile attachments on every write

A **dedicated media deletion worker** is responsible for:

* Consuming media delete events
* Iterating attachments
* Triggering per-domain detachment
* Removing attachment rows

---

## Required Platform Capabilities (Ticket-Backed)

### Canonical Attachment Linking Layer (REQUIRED)

A reusable service/helper MUST exist that:

* Accepts:

  * `entity_type`
  * `entity_id`
  * `store_id`
  * `old_media_ids[]`
  * `new_media_ids[]`
* Computes diff
* Creates and removes `media_attachments` rows accordingly
* Sets `is_protected` deterministically at creation
* Enforces store scoping
* Requires an active transaction

All domain integrations MUST use this layer.

---

## Change Management

* Any new domain that references media MUST integrate with this lifecycle.
* Any new protected entity type MUST be:

  * Added to `ProtectedAttachmentEntities`
  * Documented here
* Any deviation from these rules requires an explicit ADR.
