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

Hurrdurr understands N basic elements that it uses to build ACLs and apply
them to a gitlab instance.

1. **Member:** is a user, it is defined by the username and must exist in the
   instance of it to be able of handling it. If the declared user does not
   exists, hurrdurr will fail execution.
1. **Group:** gitlab group whose members are managed by hurrdurr.
   Hurrdurr will only manage the groups that are declared in the configuration,
   other groups will be ignored.
1. **Level:** a level setting in gitlab, known levels in increasing access
   rights are guest, reporter, developer, maintainer, owner. A group can
   declare none, or all the levels, assigning members directly or through
   queries.
1. **query:** a lazy definition of a group which is expanded into members,
   these are used to define which members should belong to a level in a given
   group.

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
    - ninja_ops_1
  owners:
    members:
    - werewolve_1
    - bohf_1
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

Queries are simple on purporse and follow strict rules.

1. You can't query a group that contains a query. This will result in a runtime error.
1. You can query for `all active regular users`, this will return the list of all the members that are not blocked or admins that exist in the remote instance.
1. You can query for a level in a group, for example: `owners in infrastructure` would return `werewolve_1, bohf_1`
1. You can query for `everyone` in a group, for example: `everyone in backend` would return `ninja_dev, samurai, ronin`
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

This will result in `pointy_haired_boss` being an owner of the handbook, and
`rrhh_demon` being a maintainer, and whomever else exists in the remote instance.