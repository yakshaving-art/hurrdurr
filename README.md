# Hurrdurr

The idiotic for herder.

Manages group acls, slowly and embarrassingly ineffectively, yet does the jerb.

## Usage

### Arguments

- **-config** the configuration file to use, by default hurrdurr will load
  *hurrdurr.yml* in the current working directoy.
- **-gitlab-url** the gitlab instance url to talk to. Can also be defined
  through the GITLAB_URL environment variable.
- **-dryrun** don't actually change anything, only evaluate which changes
  should happen.
- **-debug** enabled debug level logging.

### Required Environment Variables

- **GITLAB_TOKEN** the token to use when contacting the gitlab instance API.

## Configuration

Configuration is managed through a yaml file. This file declares the
structure of groups, levels and members for hurrdurr to colapse reality into.

### Concepts

Hurrdurr understands 4 basic elements that it uses to build ACLs and apply
them to a gitlab instance.

#### Member

A member is a user, it is defined by the username and must exist in the
instance. If the declared user does not exist, hurrdurr will fail the
execution.

#### Group

A gitlab group whose members are managed by hurrdurr. Hurrdurr will only
manage the groups that are declared in the configuration, other groups will
be ignored.

#### Level

A level setting in gitlab. The levels, sorted by decreasing access rights,
are:

- Owner
- Maintainer
- Developer
- Reporter
- Guest

#### Query

a lazy definition of a group. It is expanded into members. Read
[below](#using-queries) for the details.

### Sample

```yaml
---
root:
  path: awesomeness
  developers:
    queries:
    - all active regular users
backend:
  path: awesomeness/backend
  owners:
    members:
    - ninja_dev
    - samurai
    - ronin
infrastructure:
  path: awesomeness/infra
  guests:
    members:
    - ninja_ops_1
  owners:
    members:
    - werewolve_1
    - bofh_1
handbook:
  path: awesomeness/handbook
  reporters:
    queries:
    - everyone
  owners:
    members:
    - manager_1
    - manager_2
```

### Using Queries

Queries are simple on purporse, and follow strict rules.

1. You can't query a group that contains a query. This will result in a runtime error.
1. You can query for `all active regular users`. This will return the list of
   all the members that are not blocked or admins that exist in the remote
   instance.
1. You can query for a level in a group. For example: `owners in
   infrastructure` would return `werewolve_1, bofh_1`.
1. You can query for `everyone` in a group. For example: `everyone in
   backend` would return `ninja_dev, samurai, ronin`.
1. You can use more than one query to assign to a level.

### ACL Leveling on expansion

Every member that is defined in a group will get the higher level it could
get. For example, if a member is expanded in 2 queries for a group both as an
owner and as a developer, the user will only be defined as an owner.

This can be useful for defining things like this:

```yaml
developers:
  owners:
    members:
    - awsum_dev
managers:
  reporters:
    members:
      - rrhh_demon
  owners:
    members:
      - pointy_haired_boss
handbook:
  path: awesomeness/handbook
  developers:
    queries:
    - all active regular users
  maintainers:
    queries:
    - everyone in managers
  owners:
    members:
    - pointy_haired_boss
```

This will result in `pointy_haired_boss` being an owner of the handbook,
`rrhh_demon` being a maintainer, and whoever else is registered as an active
regular user in the remote instance to be assigned as a developer.