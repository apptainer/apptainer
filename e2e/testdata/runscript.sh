#!/bin/sh

size=`stat -c %s /.apptainer.d/runscript`
if [ "$size" == "$1" ]; then
    exit 0
else
    exit 1
fi
