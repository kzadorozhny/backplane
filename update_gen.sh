#!/bin/sh
go generate github.com/apesternikov/backplane/...

go get github.com/apesternikov/bindata/mkbinfs/
mkbinfs src/backplane/static/