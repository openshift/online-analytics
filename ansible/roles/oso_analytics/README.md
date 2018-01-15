# oso_analytics

A role to deploy and configure the OpenShift Online User Analytics application.

## Dependencies

- lib_openshift role from openshift-ansible must be loaded in a playbook prior to running this role.
- oc_start_build_check role from online-archivist repository. (included via gogitit)

## Role Variables

### Required

* osoan_cluster_name - Cluster name. (usage? TODO)
* osoan_woopra_enabled - Enable Woopra functionality.
* osoan_woopra_domain - Woopra domain to report to. (can leave blank if woopra not enabled)

### Optional

* osoan_git_repo - The git repository to deploy from.
* osoan_git_ref - A git branch, tag, or commit to deploy from the repo.
* osoan_namespace - Namespace to deploy to.
* osoan_woopra_endpoint - Woopra endpoint to talk to.
* osoan_local_endpoint_enabled - Set true to enable a test endpoint. (likely used woopra disabled in devel environments)
* osoan_user_key_strategy - TODO
* osoan_log_level
* osoan_uninstall - Set to 'true' to uninstall before running role. (can also include_role with tasks_from to just uninstall without re-installing from a playbook)

