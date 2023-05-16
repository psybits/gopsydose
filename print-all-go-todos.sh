#!/bin/bash

grep --recursive --line-number --ignore-case --word-regexp --include \*.go "todo" .
[ $? == 1 ] && echo "No TODOs were found."
