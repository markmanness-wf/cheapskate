#! /bin/bash

set -e

FRUGAL=drydock.workiva.org/workiva/frugal:35054
PREFIX=github.com/danielrowles-wf/cheapskate/gen-go/
IMPORT="github.com/Workiva/frugal/lib/go"
TOPDIR=$PWD

echo "Building parsimony"
go get github.com/Workiva/parsimony

if [ -e ./gen-go ]; then
    echo "Remove existing gen-go directory"
    rm -Rf gen-go
fi

if [ -e ./stage ]; then
    echo "Remove existing stage directory"
    rm -Rf stage
fi

echo "Fetch all required IDL files"
$GOPATH/bin/parsimony --staging stage stingy.frugal

echo "Generate GO code"
docker run -u $UID -v "$(pwd):/data" $FRUGAL frugal --gen=go:package_prefix=$PREFIX -r stage/stingy.frugal

echo "Fix missing imports"
for bork in stingy/f_stingyservice_service.go workiva_frugal_api/f_baseservice_service.go; do
    if ! grep $IMPORT ./gen-go/$bork; then
        echo "Missing <$IMPORT> in ./gen-go/$bork - add manually" >&2
        sed -i -e "s!import (!import (\n        \"$IMPORT\"!" ./gen-go/$bork;
    fi
done

for dir in cheapskate client; do
    echo "Building <$dir>"
    cd $TOPDIR/$dir
    if [ -e $dir ]; then
        rm $dir
    fi
    go build .
    cd $TOPDIR
done
