#!/bin/bash

source <(checkpoint use $(basename $0))
completed s1 || echo 1
cat does-not-exist &> /dev/null
completed s2 || echo 2
completed
