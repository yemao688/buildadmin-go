# Changelog

## v2.0.0

- **Breaking:** Removed the exported mutable auth cache globals `AuthGroupList`, `AuthRuleList`, and `AuthRuleNameList` from `app/admin/model` and `app/common/model`. Permission caches are now instance-owned, mutex-synchronized, and invalidated automatically on rule/group mutations; downstream code referencing those variables must drop the references (there is no replacement global API).
- **Breaking:** `Attachment.Admin` and `Attachment.User` in `app/common/model` now use the local `AttachmentAdmin`/`AttachmentUser` DTO types instead of `app/admin/model/simple.Admin`/`simple.User`. Field sets and JSON shapes are unchanged; downstream code naming the old types must switch to the new ones.
- **Breaking:** Introduced the RouteRegistrar routing system; `InitRouter` now receives a registrar set instead of the former 39 handwritten handler parameters.
- Changed the CRUD generator to produce module route registrar files and maintain `router/registrar_set.go`; it no longer injects route strings into `router/router.go`.
- Fixed a terminal security issue where a failed authentication check did not stop the configured command from executing.
- Corrected atomic capability key normalization and moved route collection after all registrar registrations.
- Added the 165-route golden snapshot baseline and runtime capability-to-route consistency tests.
- Added the downstream fork migration guide for converting custom modules to RouteRegistrar.

### Upgrade

See [`docs/route-registrar-migration.md`](docs/route-registrar-migration.md) for the downstream upgrade and module migration procedure.
