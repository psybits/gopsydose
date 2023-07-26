#!/bin/bash

cd drugdose && go test && cd .. && goreleaser release --snapshot
