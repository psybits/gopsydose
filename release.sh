#!/bin/bash

go test && goreleaser release --snapshot
