# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/shyiko/kubensx/compare/0.1.1...0.2.0) - 2018-04-29

### Added

- Support for users that `cannot list namespaces at the cluster scope` (per Access Control policy).
- Ability to combine `--user`(`-u`) / `--cluster`(`-c`) / `--namespace`(`--ns`,`-n`)  
(e.g. `kubensx current -cn` should print `<current cluster>/<current namespace>`).
- `kubensx use` option filtering ("type to filter"). 

## [0.1.1](https://github.com/shyiko/kubensx/compare/0.1.0...0.1.1) - 2018-01-05

### Fixed
- Interactive selection (from the list containing a single option that doesn't match "current" one). 
- Azure/GCP/OIDC/OpenStack auth ([#1](https://github.com/shyiko/kubensx/pull/1)).  

## 0.1.0 - 2018-01-01
