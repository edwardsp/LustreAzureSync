#!/bin/bash

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $ROOT_DIR

tag=$(git describe --tags --abbrev=6 --always)

if [[ $(git diff --shortstat 2> /dev/null | tail -n1) != "" ]]; then
  tag="$tag-dirty"
fi

if [[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  version="$tag"
else
  version="dev-$tag"
fi

go build -buildvcs=false -ldflags "-X main.version=$version" -o LustreAzureSync ./src/LustreAzureSync
