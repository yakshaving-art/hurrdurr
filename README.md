# Hurrdurr

The idiotic for herder.

Manages group acls, slowly and embarrassingly ineffectively, yet does the jerb.

## Usage

### Arguments

- **-config** the configuration file to use, by default hurrdurr will load
  *hurrdurr.yml* in the current working directoy.
- **-dryrun** don't actually change anything, only evaluate which changes
  should happen.
- **-version** prints the version and exits without error

### Required Environment Variables

- **GITLAB_TOKEN** the token to use when contacting the gitlab instance API.
- **GITLAB_BASEURL** the gitlab instance url to talk to.

## Configuration

Configuration is managed through a yaml file. This file declares the
structure of groups, levels and members for hurrdurr to colapse reality into.

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

### Sample

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
```

This will result in `pointy_haired_boss` being an owner of the handbook,
`rrhh_demon` being a maintainer, and whoever else is registered as an active
regular user in the gitlab instance to be assigned as a developer.
