#!/bin/bash

source <(checkpoint use $(basename $0))
trap completed EXIT
completed s1 || echo 1
if ! completed s2; then
  echo 2
fi
# NOTE, s2 will be marked as complete by the trap statement.
exit 0
