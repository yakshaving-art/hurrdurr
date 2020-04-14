#!/bin/bash
# vim: ai:ts=8:sw=8:noet
# build.sh: run build for this project, run from local machine or CI
# Usage: bash path/to/build.sh

# First, set up some healthy tensions about how this script should be used:
#   - exclusively bash. POSIX purists are invited to maintain their forks :)
#   - exclusively bash-4.4 or later.
#   - executing, not sourcing.
# This doesn't make it safe, but it makes it reasonable safe to tolerate it.
[ -n "${BASH_VERSION}" ] || { echo "Error: bash is required!" ; exit 1; }
# note: we can use [[ and || here and below
if [[ 44 -gt "${BASH_VERSINFO[0]}${BASH_VERSINFO[1]}" ]]; then
	# of course, assuming there is no v2.10 out there :)
	echo "Error: bash 4.4 or above is required!"
	exit 1
fi

if [[ "${0}" != "${BASH_SOURCE[0]}" ]]; then
	echo "Error: script ${BASH_SOURCE[0]} is not supported to be sourced!"
	return 1
fi

# Next, we're free to use bashisms, so lets set pretty strict defaults:
#  - exit on error (-e) (caveat lector)
#  - no unset variables (-u)
#  - no glob (-f)
#  - no clobber (-C)
#  - pipefail
# , propagate those to children with SHELLOPTS and set default IFS.
# Again, not ideal, but reasonably safe-ish.
set -eufCo pipefail
export SHELLOPTS
IFS=$'\t\n'

# Next, check required commands are in place, and fail fast if they are not
_cmds_missing=0
while read -r ; do
	[[ "${REPLY}" =~ ^\s*#.*$ ]] && continue	# convenient skip
	if ! command -v "${REPLY}" >/dev/null 2>&1; then
		echo "Error: please install '${REPLY}' command or use image that has it"
		_cmds_missing+=1
	fi
done <<-COMMANDS
	docker
	# shellcheck
COMMANDS
[ 0 -eq "${_cmds_missing}" ] || { exit 1; }

# Next, set up default variables, depending if we're on CI or not
if [[ "true" == "${GITLAB_CI:-false}" ]]; then
	CONTAINER_TEST_IMAGE="${CI_REGISTRY}/${CI_PROJECT_PATH}:${CI_COMMIT_REF_SLUG}"

	# On gitlabCI, just login to registry
	echo "${CI_JOB_TOKEN}" | docker login -u gitlab-ci-token --password-stdin "${CI_REGISTRY}"
else
	echo "Running not on Gitlab CI, so attempting to guess variables:"
	# assume $root_dir/.gitlab.d/ci/scripts/build.sh location:
	CI_PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.."; pwd)"

	CI_REGISTRY="${CI_REGISTRY:-registry.gitlab.com}"
	CI_PROJECT_PATH="${CI_PROJECT_PATH:-undefined}"

	# just get active branch and replace non-alphumdashes with dash
	CI_COMMIT_REF_SLUG="${CI_COMMIT_REF_SLUG:-nonCIpush-$(git branch | grep '^\*' | awk '{gsub(/[^a-z0-9-]/, "-", $2);print $2;}')}"
	CONTAINER_TEST_IMAGE="${CI_REGISTRY}/${CI_PROJECT_PATH}:${CI_COMMIT_REF_SLUG}"

	_vars_missing=0
	pad="$(printf '_%.0s' {1..32})"
	while read -r ; do
		# fancy-shmancy printing
		if [[ "undefined" == "${!REPLY}" ]]; then
			_vars_missing+=1
			printf "\t%s %s \033[0;31m* Error! \033[0m %s\n" "${REPLY}" "${pad:${#REPLY}}" "${!REPLY}"
		else
			printf "\t%s %s %s\n" "${REPLY}" "${pad:${#REPLY}}" "${!REPLY}"
		fi
	done <<-VARIABLES
		CI_COMMIT_REF_SLUG
		CI_PROJECT_DIR
		CI_PROJECT_PATH
		CI_REGISTRY
		CONTAINER_TEST_IMAGE
	VARIABLES

	if [[ 0 -ne "${_vars_missing}" ]]; then
		echo
		echo "Please export all of the undefined variables to proceed."
		exit 1
	else
		echo "Will try to run build scripts now or fail miserably, hold on to your butts."
		echo "Not trying to login to '${CI_REGISTRY}', please do so manually if needed"
	fi
fi

# Next, source whatever helpers we need
# shellcheck disable=SC1090
# source <(set +f; cat /usr/local/lib/functionarium/*) || { echo "Please install functionarium"; exit 1; }

# Next, set up all the traps
# [[ "true" == "${GITLAB_CI:-false}" ]] && trap ci_shred_secrets EXIT

# Finally, below this line is where all the actual functionality goes
#####################################################################

if [[ ! -f "${CI_PROJECT_DIR}/Dockerfile" ]]; then
	echo "Nothing to build, kthxbye!"
	exit 0
fi

# Finally, build image and push it to registry

# setup buildx
export DOCKER_CLI_EXPERIMENTAL=enabled
# NOTE: this is sometimes hangs on gitlab.com
docker run --rm --privileged docker/binfmt:a7996909642ee92942dcd6cff44b9b95f08dad64
docker buildx create --use --name mahbilda

# do things
docker buildx build \
	--pull \
	--push \
	--platform "${MUCHOS_ARCHES// /,}" \
	-t "${CONTAINER_TEST_IMAGE}-pre-test" .

# Lastly, logout
docker logout "${CI_REGISTRY}"
