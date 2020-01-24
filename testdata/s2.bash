#!/bin/bash

source <(checkpoint use $(basename $0))
completed s1 || echo 1
completed s2 || echo 2
# NOTE, s2 will not be marked as complete
exit 0
