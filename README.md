# Gitea Group Sync from LDAP

This application is designed to sync LDAP groups and user membership to Gitea.

It can do the following:

- **Create** (and optionally delete) **Gitea Organizations** based on **LDAP groups**.
- **Create** (and optionally delete) **Gitea Teams** inside Organizations based on **LDAP subgroups**.
- **Attach** existing **Gitea Users** to appropriate **Gitea Teams** based on group membership information in LDAP.

**The application is not going to sync users from LDAP to Gitea as Gitea provides a solution for that.**

Docker image available
at [ghcr.io/janosmiko/gitea-group-sync](https://github.com/janosmiko/gitea-group-sync/pkgs/container/gitea-group-sync).

## Usage

### Docker Compose

Configure your settings in `docker-compose.yml` or copy `config.yaml.sample` as `config.yaml` and fill the settings (and uncomment the volume mount in docker-compose.yml`.

```
docker-compose up -d
```

### Kubernetes

Modify the values in [deploy/secret.yaml](deploy/secret.yaml) and [deploy/job.yaml](deploy/job.yaml) and apply them to Kubernetes.

```
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment.yaml
```

## Configuration Options

You can configure the application using a `yaml` config file (find a sample in this repository) or using Environment
Variables.

Available Environment Variables (find example values in [config.yaml.sample](config.yaml.sample)):

| Variable                       | Description                                                | Default            |
|--------------------------------|------------------------------------------------------------|--------------------|
| `DEBUG`                        | Enable debug mode                                          | `false`            |
| `GITEA_BASE_URL`               | Gitea baseURL in `https://user@gitea.com` format.          | `""`               |
| `GITEA_TOKEN`                  | Gitea admin user token                                     | `""`               |
| `LDAP_URL`                     | LDAP connection URL                                        | `""`               |
| `LDAP_PORT`                    | LDAP connection port                                       | `389`              |
| `LDAP_USE_TLS`                 | Enable TLS connection for LDAP                             | `true`             |
| `LDAP_ALLOW_INSECURE_TLS`      | Allow insecure TLS connections (disable cert verification) | `false`            |
| `LDAP_BIND_DN`                 | LDAP Bind DN (or username)                                 | `""`               |
| `LDAP_BIND_PASSWORD`           | LDAP Bind Password                                         | `""`               |
| `LDAP_USER_SEARCH_BASE`        | LDAP User Search Base                                      | `""`               |
| `LDAP_USER_FILTER`             | LDAP User Filter                                           | `""`               |
| `LDAP_USER_IDENTITY_ATTRIBUTE` | LDAP attribute to match Gitea User                         | `"sAMAccountName"` |
| `LDAP_USER_FULLNAME`           | LDAP attribute for Gitea User Fullname                     | `"cn"`             |
| `LDAP_GROUP_SEARCH_BASE`       | LDAP Group Search Base (Gitea Organizations)               | `""`               |
| `LDAP_GROUP_FILTER`            | LDAP Group Filter                                          | `""`               |
| `LDAP_SUBGROUP_SEARCH_BASE`    | LDAP Subgroup Search Base (Gitea Teams)                    | `""`               |
| `LDAP_SUBGROUP_FILTER`         | LDAP Subgroup filter                                       | `""`               |
| `LDAP_EXCLUDE_GROUPS`          | Exclude groups from sync (separated by whitespace)         | `""`               |
| `LDAP_EXCLUDE_GROUPS_REGEX`    | Exclude groups from sync (regular expression)              | `""`               |
| `LDAP_EXCLUDE_SUBGROUPS`       | Exclude subgroups from sync (separated by whitespace)      | `""`               |
| `LDAP_EXCLUDE_SUBGROUPS_REGEX` | Exclude groups from sync (regular expression)              | `""`               |
| `LDAP_TRIM_PARENT_NAME`        | Trim parent name from subgroup name                        | `false`            |
| `LDAP_SUBGROUP_SEPARATOR`      | Trim parent name from subgroup name by this separator      | `"/"`              |
| `CRON_TIMER`                   | Configure the schedule of the sync (cron format)           | `"@every 1m"`      |
| `SYNC_CONFIG_CREATE_GROUPS`    | Create non-existing groups in Gitea.                       | `true`             |
| `SYNC_CONFIG_FULL_SYNC`        | Delete groups from Gitea if they are not existing in LDAP  | `false`            |


Additional settings for creating Organizations and Teams in Gitea:
- `SYNC_CONFIG_DEFAULTS_ORGANIZATION_REPO_ADMIN_CHANGE_TEAM_ACCESS`
- `SYNC_CONFIG_DEFAULTS_ORGANIZATION_VISIBILITY`
- `SYNC_CONFIG_DEFAULTS_TEAM_CAN_CREATE_ORG_REPO`
- `SYNC_CONFIG_DEFAULTS_TEAM_INCLUDES_ALL_REPOSITORIES`
- `SYNC_CONFIG_DEFAULTS_TEAM_PERMISSION`
- `SYNC_CONFIG_DEFAULTS_TEAM_UNITS`
- `SYNC_CONFIG_DEFAULTS_TEAM_UNITS_MAP`

# License

This work is licensed under the MIT license. See LICENSE file for details.

# Acknowledgement

This project is based on the idea by [Gitea Group Sync by TWS Inc](https://github.com/gitea-group-sync/gitea-group-sync)
.