## [2.3.9](https://github.com/pdf/zfs_exporter/compare/v2.3.8...v2.3.9) (2025-08-24)


### Bug Fixes

* **props:** Add support for dataset `creation` property ([a1c90f4](https://github.com/pdf/zfs_exporter/commit/a1c90f4)), closes [#57](https://github.com/pdf/zfs_exporter/issues/57)




## [2.3.8](https://github.com/pdf/zfs_exporter/compare/v2.3.7...v2.3.8) (2025-04-20)


### Bug Fixes

* **build:** Bump Go version and golangci-lint ([4d46ab3](https://github.com/pdf/zfs_exporter/commit/4d46ab3))




## [2.3.7](https://github.com/pdf/zfs_exporter/compare/v2.3.6...v2.3.7) (2025-04-20)


### Bug Fixes

* **deps:** Bump dependencies ([6af54d2](https://github.com/pdf/zfs_exporter/commit/6af54d2))




## [2.3.6](https://github.com/pdf/zfs_exporter/compare/v2.3.5...v2.3.6) (2025-01-18)


### Bug Fixes

* **build:** Bump Go version in actions ([00498df](https://github.com/pdf/zfs_exporter/commit/00498df))




## [2.3.5](https://github.com/pdf/zfs_exporter/compare/v2.3.4...v2.3.5) (2025-01-18)


### Bug Fixes

* **core:** Bump dependencies, migrate to promslog ([ccc2b21](https://github.com/pdf/zfs_exporter/commit/ccc2b21))




## [2.3.4](https://github.com/pdf/zfs_exporter/compare/v2.3.3...v2.3.4) (2024-04-13)


### Bug Fixes

* **deps:** Bump deps for security ([1404536](https://github.com/pdf/zfs_exporter/commit/1404536))




## [2.3.3](https://github.com/pdf/zfs_exporter/compare/v2.3.2...v2.3.3) (2024-04-13)


### Bug Fixes

* **log:** Improve command execution error output ([2277832](https://github.com/pdf/zfs_exporter/commit/2277832))




## [2.3.2](https://github.com/pdf/zfs_exporter/compare/v2.3.1...v2.3.2) (2023-10-13)




## [2.3.1](https://github.com/pdf/zfs_exporter/compare/v2.3.0...v2.3.1) (2023-08-12)


### Bug Fixes

* **build:** Update deps ([ddf8e09](https://github.com/pdf/zfs_exporter/commit/ddf8e09))




# [2.3.0](https://github.com/pdf/zfs_exporter/compare/v2.2.8...v2.3.0) (2023-08-12)


### Features

* **server:** Add exporter toolkit for TLS support ([8102e2e](https://github.com/pdf/zfs_exporter/commit/8102e2e)), closes [#34](https://github.com/pdf/zfs_exporter/issues/34)




## [2.2.8](https://github.com/pdf/zfs_exporter/compare/v2.2.7...v2.2.8) (2023-04-22)


### Bug Fixes

* **build:** Tag correct commit SHA ([0712333](https://github.com/pdf/zfs_exporter/commit/0712333))
* **security:** Update dependencies for upstream vulnerabilities ([2220da2](https://github.com/pdf/zfs_exporter/commit/2220da2))




## [2.2.7](https://github.com/pdf/zfs_exporter/compare/v2.2.6...v2.2.7) (2023-01-28)


### Bug Fixes

* **transform:** Add support for ancient ZFS dedupratio metric ([85bdc3b](https://github.com/pdf/zfs_exporter/commit/85bdc3b)), closes [#26](https://github.com/pdf/zfs_exporter/issues/26)




## [2.2.6](https://github.com/pdf/zfs_exporter/compare/v2.2.5...v2.2.6) (2023-01-28)


### Bug Fixes

* **transform:** Add support for ancient ZFS fragmentation metric ([a0240d1](https://github.com/pdf/zfs_exporter/commit/a0240d1)), closes [#26](https://github.com/pdf/zfs_exporter/issues/26)




## [2.2.5](https://github.com/pdf/zfs_exporter/compare/v2.2.4...v2.2.5) (2022-01-30)


### Bug Fixes

* **core:** Correctly handle and report errors listing pools ([efbcceb](https://github.com/pdf/zfs_exporter/commit/efbcceb)), closes [#18](https://github.com/pdf/zfs_exporter/issues/18)




## [2.2.4](https://github.com/pdf/zfs_exporter/compare/v2.2.3...v2.2.4) (2022-01-05)


### Bug Fixes

* **build:** Update promu config to build v2 ([2a38914](https://github.com/pdf/zfs_exporter/commit/2a38914))




## [2.2.3](https://github.com/pdf/zfs_exporter/compare/v2.2.2...v2.2.3) (2022-01-05)


### Bug Fixes

* **build:** update go module version to match release tag major version ([f709083](https://github.com/pdf/zfs_exporter/commit/f709083))




## [2.2.2](https://github.com/pdf/zfs_exporter/compare/v2.2.1...v2.2.2) (2021-11-16)


### Bug Fixes

* **metrics:** Fix typo in metric name ([bbd3d91](https://github.com/pdf/zfs_exporter/commit/bbd3d91))
* **pool:** Add SUSPENDED status ([9b9e655](https://github.com/pdf/zfs_exporter/commit/9b9e655))
* **tests:** Remove unnecessary duration conversion ([b6a29ab](https://github.com/pdf/zfs_exporter/commit/b6a29ab))




## [2.2.1](https://github.com/pdf/zfs_exporter/compare/v2.2.0...v2.2.1) (2021-09-13)


### Bug Fixes

* **collector:** Avoid race on upstream channel close, tidy sync points ([e6fbdf5](https://github.com/pdf/zfs_exporter/commit/e6fbdf5))
* **docs:** Document web.disable-exporter-metrics flag in README ([20182da](https://github.com/pdf/zfs_exporter/commit/20182da))




# [2.2.0](https://github.com/pdf/zfs_exporter/compare/v2.1.1...v2.2.0) (2021-09-04)


### Bug Fixes

* **docs:** Correct misspelling ([066c7d2](https://github.com/pdf/zfs_exporter/commit/066c7d2))


### Features

* **metrics:** Allow disabling exporter metrics ([1ca8717](https://github.com/pdf/zfs_exporter/commit/1ca8717)), closes [#2](https://github.com/pdf/zfs_exporter/issues/2)




## [2.1.1](https://github.com/pdf/zfs_exporter/compare/v2.1.0...v2.1.1) (2021-08-27)


### Bug Fixes

* **build:** Update to Go 1.17 for crossbuild, and enable all platforms ([f47b69a](https://github.com/pdf/zfs_exporter/commit/f47b69a))
* **core:** Update dependencies ([b39382b](https://github.com/pdf/zfs_exporter/commit/b39382b))




# [2.1.0](https://github.com/pdf/zfs_exporter/compare/v2.0.0...v2.1.0) (2021-08-18)


### Bug Fixes

* **logging:** Include collector in warning for unsupported properties ([1760a4a](https://github.com/pdf/zfs_exporter/commit/1760a4a))
* **metrics:** Invert ratio for multiplier fields, and clarify their docs ([1a7bc3a](https://github.com/pdf/zfs_exporter/commit/1a7bc3a)), closes [#11](https://github.com/pdf/zfs_exporter/issues/11)


### Features

* **build:** Update to Go 1.17 ([b64115c](https://github.com/pdf/zfs_exporter/commit/b64115c))




# [2.0.0](https://github.com/pdf/zfs_exporter/compare/v1.0.1...v2.0.0) (2021-08-14)


### Code Refactoring

* **collector:** Migrate to internal ZFS CLI implementation ([53b0e98](https://github.com/pdf/zfs_exporter/commit/53b0e98)), closes [#7](https://github.com/pdf/zfs_exporter/issues/7) [#9](https://github.com/pdf/zfs_exporter/issues/9) [#10](https://github.com/pdf/zfs_exporter/issues/10)


### Features

* **performance:** Execute collection concurrently per pool ([ccc6f22](https://github.com/pdf/zfs_exporter/commit/ccc6f22))
* **zfs:** Add local ZFS CLI parsing ([f5050b1](https://github.com/pdf/zfs_exporter/commit/f5050b1))


### BREAKING CHANGES

* **collector:** Ratio values are now properly calculated in the range
0-1, rather than being passed verbatim.

The following metrics are affected by this change:
- zfs_pool_deduplication_ratio
- zfs_pool_capacity_ratio
- zfs_pool_fragmentation_ratio
- zfs_dataset_compression_ratio
- zfs_dataset_referenced_compression_ratio

Additionally, the zfs_dataset_fragmentation_percent metric has been
renamed to zfs_dataset_fragmentation_ratio.




## [1.0.1](https://github.com/pdf/zfs_exporter/compare/v1.0.0...v1.0.1) (2021-08-03)


### Bug Fixes

* fix copy and paste errors when accessing dataset properties ([c0fc6b2](https://github.com/pdf/zfs_exporter/commit/c0fc6b2))




# [1.0.0](https://github.com/pdf/zfs_exporter/compare/v0.0.3...v1.0.0) (2021-06-22)


### Bug Fixes

* **ci:** Fix syntax error in github actions workflow ([0b6e8bc](https://github.com/pdf/zfs_exporter/commit/0b6e8bc))


### Code Refactoring

* **core:** Update prometheus toolchain and refactor internals ([056b386](https://github.com/pdf/zfs_exporter/commit/056b386))


### Features

* **enhancement:** Allow excluding datasets by regular expression ([8dd48ba](https://github.com/pdf/zfs_exporter/commit/8dd48ba)), closes [#3](https://github.com/pdf/zfs_exporter/issues/3)


### BREAKING CHANGES

* **core:** Go API has changed somewhat, but metrics remain
unaffected.




