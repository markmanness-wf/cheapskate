#! /bin/bash

LEN=$[ RANDOM % 10 ]
CODE=$[ RANDOM % 5 ]

echo "All Ur Health Are Belong To DevOps <$LEN> <$CODE>"

sleep $LEN

exit $CODE
