# HurrDurr

The idiotic for herder.

Manages user, group and project acls, slowly and embarrassingly
ineffectively, yet does the jerb.

## Usage

### Arguments

- **-autodevopsmode** where you have no admin rights but still do what you
  gotta do.
- **-config** the configuration file to use, by default HurrDurr will load
  *hurrdurr.yml* in the current working directory.
- **-checksum-check** validates the configuration checksum reading it from a
  file called as the configuration file ended in `.md5`.
- **-dryrun** don't actually change anything, only evaluates which changes
  should happen.
- **-ghost-user** system wide GitLab ghost user. (default "ghost")
- **-manage-acl** manage groups, projects permissions and sharing.
- **-manage-users** manage user properties, like adminness and blockedness.
- **-snoopdepth** do not report unmanaged groups located deeper than this.
- **-version** prints the version and exits without error.
- **-yolo-force-secrets-overwrite** life is too short to not overwrite group
  and project environment variables.

### Required Environment Variables

- **GITLAB_TOKEN** the token to use when contacting the GitLab instance API.
- **GITLAB_BASEURL** the GitLab instance url to talk to.

### API Token scope

You'll want to generate or re-use a token with just the `api` scope.

### AutoDevOpsMode and where to use it

If the token you are using with HurrDurr belongs to an *Admin* user on
your GitLab instance, you don't need the *AutoDevOpsMode* mode.

If you plan to use HurrDurr on an instance that you are not managing
then you probably want to use the *AutoDevOpsMode* flag.
In this particular case, HurrDurr will lazily load users and projects
to avoid fetching the whole universe at once.

## Configuration

Configuration is managed through a yaml file. This file declares the
structure of groups, project, levels and members for HurrDurr to collapse
reality into.

### Concepts

HurrDurr understands 7 basic elements that it uses to build ACLs and apply
them to a GitLab instance.

- #### Member

  A member is a user, it is defined by the username and must exist in the
  instance. If the declared user does not exist, HurrDurr will fail the
  execution.

- #### Group

  A GitLab group whose members are managed by HurrDurr. HurrDurr will only
  manage the groups that are declared in the configuration, other groups will
  be ignored.

- #### Project

  A GitLab project can be shared with a group at any ACL level. Projects can
  also have members added to it the same way we do with groups.

- #### Level

  A level setting in GitLab. The levels, sorted by decreasing access rights,
  are:

  - Owner
  - Maintainer
  - Developer
  - Reporter
  - Guest

- #### Query

  A lazy definition of a group. It is expanded into members. Read
  [below](#using-queries) for the details.

- #### User

  A GitLab user that is being specifically managed by HurrDurr. They can
  either be set admins, or can be blocked.

- #### Bots

  A Gitlab user that is automatically created by hurrdurr, with a random
  password to be used as an unpersonalized bot user.

### Full Sample

```yaml
---
groups:
  root:
    developers:
    - "query: users"
  backend:
    owners:
    - ninja_dev
    - samurai
    - ronin
  infrastructure:
    owners:
    - werewolve_1
    - bofh_1
    guests:
    - ninja_ops_1
  handbook:
    owners:
    - manager_1
    - manager_2
    reporters:
    - "query: users"
    - "query: owners from backend"
  runbook:
    owners:
    - "share_with: handbook"
    reporters:
    - "share_with: handbook"
projects:
  infrastructure/myproject:
    guests:
    - backend
users:
  admins:
  - manager_1
  blocked:
  - bad_actor_1
bots:
  - username: bot1
    email: bot1@amazeballs.skills
files:
  - moar_projects.yml
  - moar_groups.yml
```

#### Additional files

Hurrdurr supports using secondary files to configure any of the blocks.

This can be done using the files list. The way it works is hurrdurr loads the
initial configuration file, and then it loads the rest of the files into the
same configuration structure in the order they are added to the list,
overriding any previous value.

There is no support of splat expansions whatsoever, names of files have to
exact.

### Using Queries

Queries are simple on purporse, and follow strict rules.

1. You can't query a group that contains a query. This will result in a runtime error.
1. You can query for `users`. This will return the list of all the
   members that are not blocked or admins that exist in the GitLab instance.
1. You can query for `admins`. This will return the list of all the
   members that are not blocked admins that exist in the GitLab instance.
1. You can query for a level in a group. For example: `owners in
   infrastructure` would return `werewolve_1, bofh_1`.
1. You can query for `users` in a group. For example: `users in
   backend` would return `ninja_dev, samurai, ronin`.
1. You can use more than one query to assign to a level.

### User ACL management

Users management has to be explicitly enabled using `-manage-users` argument.
Once enabled, users will be managed the following way:

1. Every user in the `admins` list will be set an admin.
1. Every user in the `blocked` list will be blocked.

### Project ACL management

Project specific ALCs can be managed in 2 ways:

1. By applying specific ACLs the same way we do with groups. By both
   declaring specific people at specific levels, or using query expansions.
1. By sharing the project with a given group at a specific level. This will
   result in the whole group having access to the project at the shared level.

### ACL Leveling on expansion

Every member that is defined in a group will get the higher level it could
get. For example, if a member is expanded in 2 queries for a group both as an
owner and as a developer, the user will only be defined as an owner.

This can be useful for defining things like this:

```yaml
groups:
  developers:
    owners:
    - awsum_dev
  managers:
    reporters:
    - rrhh_demon
    owners:
    - pointy_haired_boss
  handbook:
    developers:
    - "query: users"
    maintainers:
    - "query: users in managers"
    owners:
    - "share_with: managers"
  rrhh:
    owners:
    - rrhh_demon
projects:
  rrhh/lobby:
    guests:
    - "share_with: developers"
    reporters:
    - "query: reporters in managers"
    owners:
    - pointy_haired_boss
```

### Group/Project secret variable management

HurrDurr can grab secret variables from one location and update them under
project or group CI settings. It understands the following configuration:

```yaml
groups:
  group/subgroup:
    secret_variables:
      GITLAB_DST_VAR: HURRDURR_SRC_VAR
projects:
  group/subgroup/project:
    secret_variables:
      GITLAB_DST_VAR_2: HURRDURR_SRC_VAR_2
```

In this configuration, HurrDurr will try to do the following:

- for subgroup: lookup environmental variable `HURRDURR_SRC_VAR`,
   and set its _value_ as a secret variable named `GITLAB_DST_VAR` at subgroup level.
- for project: lookup environmental variable `HURRDURR_SRC_VAR_2`,
   and set its _value_ as a secret variable named `GITLAB_DST_VAR_2` at project level.

It is up to gitlab operator to figure out priorities of those variables and
design acceptable overrides.

#### Error handling

- If there's no `HURRDURR_SRC_VAR` set in the HurrDurr environment, it will
  fail fast without making any changes. Dry run mode will complain about
  it and return non-zero code.

- If the `HURRDURR_SRC_VAR` already exists:
  - if the values match, then HurrDurr will do nothing.
  - if the values don't match, then HurrDurr will exit with error, unless `-yolo-force-secrets-overwrite` is given,
    in which case it will overwrite the variable with neither hesitation nor backups, as life's to short for that crap.

### Managing Bot users

Hurrdurr supports creating bot users to provide depersonalized accounts for automations.

To create a bot you will need to use the `-manage-bots` flag, in conjuntion
with the `BOT_USERNAME_REGEX` environment variable.

This variable is used to enforce some sanity into these users to make them easy to spot.

Just define this variable, enable the flag, and create as many bots as you
want. They will not require an email validation, will not have 2FA enabled
(there is nothing in the API to do it), and will have a random password set.

Just login as an admin, impersonate, and configure them.
