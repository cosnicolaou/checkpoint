#!/bin/bash

source <(checkpoint use $(basename $0))
completed s1 || echo 1
completed s2 || echo 2
completed s3 || echo 3
completed
