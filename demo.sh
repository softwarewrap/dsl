#!/usr/bin/env bash

docker run -it --rm -v "$(pwd):/demo" -e DSL_FILE=/demo/dsl.yaml -e SCHEMA_FILE=/demo/schema.json -e INDEX_DIR=/demo/index teherr/ttt_demo
