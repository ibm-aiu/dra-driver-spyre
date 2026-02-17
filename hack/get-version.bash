#!/bin/bash
# +-------------------------------------------------------------------+
# | Copyright IBM Corp. 2025 All Rights Reserved                      |
# | PID 5698-SPR                                                      |
# +-------------------------------------------------------------------+

set -e
function usage() {
	echo "Usage:   get-version.sh current-version"
	exit 2
}
function branch_name_check() {
	local branch_name=${1}
	local current_version=${2}
	local hash=${3}
	if [[ ${branch_name} =~ ^release_[0-9]+(\_[0-9]+)+$ ||
		${branch_name} =~ ^release_v[0-9]+(\.[0-9]+)+$ ||
		${branch_name} =~ ^v[0-9](\.[0-9]+)+-rc\.[0-9]+$ ]]; then
		echo ${current_version}
	elif [[ ${branch_name} =~ ^v[0-9]+\.[0-9]+$ ||
		${branch_name} =~ ^update_to_v[0-9]+(\.[0-9]+)+$ ||
		${branch_name} == "main" ]]; then
		echo ${current_version}-dev
	else
		echo ${current_version}-dev-${hash}
	fi
}
function use_git() {
	local current_version=${1}
	local short_hash=$(git rev-parse --short=7 HEAD)
	local branch_name=$(git branch --show-current)
	if [[ -z ${branch_name} ]]; then
		branch_name=$(git rev-parse --abbrev-ref HEAD)
	fi
	branch_name_check ${branch_name} ${current_version} ${short_hash}
}

if [[ $1 == "" ]]; then
	usage
fi

use_git ${1}
