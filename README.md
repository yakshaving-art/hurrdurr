# Hurrdurr

The idiotic for herder.

Manages user, group and project acls, slowly and embarrassingly
ineffectively, yet does the jerb.

## Usage

### Arguments

- **-config** the configuration file to use, by default hurrdurr will load
  *hurrdurr.yml* in the current working directoy.
- **-dryrun** don't actually change anything, only evaluate which changes
  should happen.
- **-version** prints the version and exits without error.
- **-ghost-user** system wide gitlab ghost user. (default "ghost")
- **-manage-users** enables user management, enforcing admins and blocked
  users to be converged.

### Required Environment Variables

- **GITLAB_TOKEN** the token to use when contacting the gitlab instance API.
- **GITLAB_BASEURL** the gitlab instance url to talk to.

## Configuration

Configuration is managed through a yaml file. This file declares the
structure of groups, project, levels and members for hurrdurr to colapse
reality into.

### Concepts

Hurrdurr understands 4 basic elements that it uses to build ACLs and apply
them to a gitlab instance.

* #### Member

  A member is a user, it is defined by the username and must exist in the
  instance. If the declared user does not exist, hurrdurr will fail the
  execution.

* #### Group

  A gitlab group whose members are managed by hurrdurr. Hurrdurr will only
  manage the groups that are declared in the configuration, other groups will
  be ignored.

* #### Project

  A gitlab project can be shared with a group at any ACL level. Projects can
  also have members added to it the same way we do with groups.

* #### Level

  A level setting in gitlab. The levels, sorted by decreasing access rights,
  are:

  - Owner
  - Maintainer
  - Developer
  - Reporter
  - Guest

* #### Query

  A lazy definition of a group. It is expanded into members. Read
  [below](#using-queries) for the details.

* #### User

 A gitlab user that is being specifically managed by hurrdurr. They can
 either be set admins, or can be blocked.

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
projects:
  infrastructure/myproject:
    guests:
    - backend
users:
  admins:
  - manager_1
  blocked:
  - bad_actor_1
```

### Using Queries

Queries are simple on purporse, and follow strict rules.

1. You can't query a group that contains a query. This will result in a runtime error.
1. You can query for `users`. This will return the list of all the
   members that are not blocked or admins that exist in the gitlab instance.
1. You can query for `admins`. This will return the list of all the
   members that are not blocked admins that exist in the gitlab instance.
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
    - "query: owners in managers"
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
