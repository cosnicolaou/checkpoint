#!/bin/bash

source <(checkpoint use $(basename $0))
completed s1 || echo 1
completed s2 || echo 2
cat does-not-exist &> /dev/null
# s2 will not be marked as complete becase the cat command fails
completed
# avoid gosh panic'ing if the script as a whole fails.
exit 0