#!/bin/bash

source <(checkpoint use $(basename $0))
step s1 && echo 1
step s2 && echo 2
# NOTE, s2 will not be marked as complete
exit 0
