#!/bin/bash

# Copyright 2014 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
API_DIR="${SCRIPT_ROOT}/api"
DEST_DIR="${SCRIPT_ROOT}/generated"
GOPKG="sigs.k8s.io/service-apis"
boilerDir="${SCRIPT_ROOT}/hack/boilerplate"

export GOFLAGS=-mod=vendor

# echo Running deepcopy-gen
# go run k8s.io/code-generator/cmd/deepcopy-gen \
#     --go-header-file ${boilerDir}/boilerplate.go.txt \
#     --input-dirs "sigs.k8s.io/service-apis/api/v1alpha1" \
#     --output-file-base zz_generated.deepcopy \
# 	--output-package ${GOPKG}/api/generated/clientset

echo Generating clientset...
go run k8s.io/code-generator/cmd/client-gen \
    --go-header-file ${boilerDir}/boilerplate.go.txt \
	--output-package ${GOPKG}/api/generated/clientset \
	--clientset-name versioned \
    --input-base "${GOPKG}" \
    --input-dirs "sigs.k8s.io/service-apis/api/v1alpha1" \
    --input "networking.x.k8s.io/v1alpha1"

echo Generating listers...
go run k8s.io/code-generator/cmd/lister-gen \
    --go-header-file ${boilerDir}/boilerplate.go.txt \
    --input-dirs "${GOPKG}/api/v1alpha1" \
	--output-package ${GOPKG}/api/generated/listers


echo Generating informers...
go run k8s.io/code-generator/cmd/informer-gen \
    --go-header-file ${boilerDir}/boilerplate.go.txt \
    --input-dirs "${GOPKG}/api/v1alpha1" \
    --versioned-clientset-package ${GOPKG}/api/generated/clientset/versioned \
	--listers-package ${GOPKG}/api/generated/listers \
	--output-package ${GOPKG}/api/generated/informers
