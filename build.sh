#! /bin/bash

set -e

FRUGAL=drydock.workiva.org/workiva/frugal:35054
PREFIX=github.com/danielrowles-wf/cheapskate/gen-go/
FILES="stingy.frugal base_service.frugal base_model.frugal"
IMPORT="github.com/Workiva/frugal/lib/go"
TOPDIR=$PWD

for file in $FILES; do
    if [ ! -e $file ]; then
        echo "Missing file <$file>\n" >&2
        exit 1
    fi
done

if [ -e ./gen-go ]; then
    echo "Remove existing gen-go directory"
    rm -Rf gen-go
fi

for file in $FILES; do
    echo "Generate <$file>"
    docker run -u $UID -v "$TOPDIR:/data" $FRUGAL frugal --gen=go:package_prefix=$PREFIX $file
done

for bork in stingy/f_stingyservice_service.go workiva_frugal_api/f_baseservice_service.go; do
    if ! grep $IMPORT ./gen-go/$bork; then
        echo "Missing <$IMPORT> in ./gen-go/$bork"
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
