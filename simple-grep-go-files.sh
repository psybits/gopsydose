#!/bin/bash

# With no arguments, the script looks 
# for all "todo" words in the Go source files.
# If an argument is added: ./simple-grep-go-files.sh word
# "word" will be used instead of "todo".

SEARCH_WORD="todo"
if [[ -n "$1" ]]; then
	SEARCH_WORD="$1"
fi

grep --recursive --line-number --ignore-case --word-regexp \
	--include '*.go' $SEARCH_WORD .

[ $? == 1 ] && echo "Search term not found in any file."
