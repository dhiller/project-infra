#!/usr/bin/env bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2023 Red Hat, Inc.
#
#

# we use the untagged version since we don't want the git tags to appear in the image tag
git_commit=$(git describe --always --dirty)
build_date=$(date -u '+%Y%m%d')
docker_tag="v${build_date}-${git_commit}"

cat <<EOF
DOCKER_TAG ${docker_tag}
EOF
